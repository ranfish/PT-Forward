package api

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.490: PTGen 三基建音乐/影视分流锚——音乐种（music 字段在场）
// 跳过 PTGen 语义链（污染/误判/白截三面）。bombPTGen 被调用即失败
// （音乐路径必须短路在 ptgen 访问之前）。
type bombPTGen struct {
	t     *testing.T
	calls int
}

func (b *bombPTGen) AnalyzePTGen(ctx context.Context, name string) (*model.PTGenResult, error) {
	b.calls++
	return nil, context.Canceled // 若被调用即视为未短路
}

func (b *bombPTGen) AnalyzePTGenForce(ctx context.Context, name string) (*model.PTGenResult, error) {
	b.calls++
	return nil, context.Canceled
}

// ptgenViable：音乐种恒 true（ptgen 未被调）；影视种正常走判定链。
func TestPTGenViableMusicBypass(t *testing.T) {
	musicJSON := `{"title":"t","music":{"artist":"a","album":"b"}}`
	bomb := &bombPTGen{t: t}
	h := &PublishTorrentsHandler{ptgen: bomb, logger: zap.NewNop()}

	// 音乐种：短路 true·ptgen 零调用
	meta := &model.TorrentMetadata{
		InfoHash:         "3a8b93e4e67c",
		Title:            "[2005.04.27] School Rumble School Running",
		DetailSourceJSON: musicJSON,
	}
	if !h.ptgenViable(context.Background(), meta) {
		t.Fatal("music seed: ptgenViable should be true (music data quality is self-sufficient)")
	}
	if bomb.calls != 0 {
		t.Fatalf("music seed: ptgen should not be called, got %d calls", bomb.calls)
	}

	// 影视种：照走判定链（title 可搜键在场→ptgen 被调→此处返回 false
	// 因 bomb 返回错误——证明未走音乐短路）
	videoMeta := &model.TorrentMetadata{InfoHash: "abc", Title: "Some Movie 2020"}
	if h.ptgenViable(context.Background(), videoMeta) {
		t.Fatal("video seed: bomb ptgen returns error, viable should be false (proves no music short-circuit)")
	}
	if bomb.calls == 0 {
		t.Fatal("video seed: ptgen should be consulted")
	}
}

// runMainlinePTGen：音乐种 return false（ptgen 未被调·不产生污染写）。
func TestRunMainlinePTGenMusicBypass(t *testing.T) {
	musicJSON := `{"title":"t","music":{"artist":"a","album":"b"}}`
	bomb := &bombPTGen{t: t}
	h := &PublishTorrentsHandler{ptgen: bomb, logger: zap.NewNop()}

	meta := &model.TorrentMetadata{
		InfoHash:         "3a8b93e4e67c",
		Title:            "[2005.04.27] School Rumble School Running",
		SiteName:         "皇后",
		DetailSourceJSON: musicJSON,
	}
	if h.runMainlinePTGen(context.Background(), meta, false) {
		t.Fatal("music seed: runMainlinePTGen should return false (skip)")
	}
	if bomb.calls != 0 {
		t.Fatalf("music seed: ptgen should not be called, got %d calls", bomb.calls)
	}
}
