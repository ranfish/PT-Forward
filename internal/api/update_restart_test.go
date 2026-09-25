package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.281: OTA 任务门控——BusyTaskDesc 空闲态
func TestBusyTaskDescIdle(t *testing.T) {
	h := &PublishTorrentsHandler{}
	h.siteBatch.tasks = map[string]*siteBatchTask{}
	h.siteBatch.active = map[string]string{}
	if d := h.BusyTaskDesc(); d != "" {
		t.Errorf("空闲态应空, got %q", d)
	}
}

// §59.286: 主链 PTGen 三级查询键列表（空键跳过）
func TestMainlinePTGenQueryKeys(t *testing.T) {
	cases := []struct {
		douban, imdb, title string
		want                []string
	}{
		{"https://douban.com/s/1", "https://imdb.com/tt1", "T", []string{"https://douban.com/s/1", "https://imdb.com/tt1", "T"}},
		{"", "https://imdb.com/tt1", "T", []string{"https://imdb.com/tt1", "T"}}, // Diesel 案
		{"", "", "Diesel 2025", []string{"Diesel 2025"}},
		{"", "", "", nil},
	}
	for _, c := range cases {
		got := mainlineQueryKeys(c.douban, c.imdb, c.title)
		if len(got) != len(c.want) {
			t.Errorf("got %v want %v", got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("got %v want %v", got, c.want)
			}
		}
	}
}

// §59.286: 三级链短路——douban 失败落 imdb 成功（stub PTGenAnalyzer）
type stubPTGen struct {
	fail map[string]bool
}

func (s *stubPTGen) AnalyzePTGen(_ context.Context, q string) (*model.PTGenResult, error) {
	if s.fail[q] {
		return nil, fmt.Errorf("stub fail")
	}
	return &model.PTGenResult{RawBBCode: "[desc]", PosterURL: "https://img.doubanio.com/p.jpg"}, nil
}
func (s *stubPTGen) AnalyzePTGenForce(ctx context.Context, q string) (*model.PTGenResult, error) {
	return s.AnalyzePTGen(ctx, q)
}

func TestMainlinePTGenChainShortCircuit(t *testing.T) {
	h := &PublishTorrentsHandler{ptgen: &stubPTGen{fail: map[string]bool{"https://douban.com/x": true}}}
	keys := mainlineQueryKeys("https://douban.com/x", "https://imdb.com/tt1", "")
	var hit *model.PTGenResult
	for _, k := range keys {
		r, err := h.ptgen.AnalyzePTGen(context.Background(), k)
		if err == nil && r != nil && r.RawBBCode != "" {
			hit = r
			break
		}
	}
	if hit == nil || hit.PosterURL == "" {
		t.Errorf("链未在 imdb 级短路: %+v", hit)
	}
}
