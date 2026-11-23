package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
)

// §59.559 PR-E: Execute 编排分支特征锚——拆分产物（PR-5 stages+PR-C upload）
// 的判定单元锁定·防后续 Extract Method 行为漂移。

// TestMusicMetaOfMeta §59.485: 音乐种独立发布分支入口——detail_source_json
// 含 music 对象即音乐种（executeMusicPublish 分流门）。
func TestMusicMetaOfMeta(t *testing.T) {
	// 音乐种：detail JSON 含 music
	musicMeta := &model.TorrentMetadata{
		DetailSourceJSON: `{"music":{"artist":"Taylor Swift","album":"1989"}}`,
	}
	if mm := musicMetaOfMeta(musicMeta); mm == nil {
		t.Fatal("音乐 JSON 应非 nil")
	} else if mm.Artist != "Taylor Swift" {
		t.Fatalf("artist: %q", mm.Artist)
	}
	// 影视种：无 music 键
	videoMeta := &model.TorrentMetadata{DetailSourceJSON: `{"douban":{"title":"Movie"}}`}
	if mm := musicMetaOfMeta(videoMeta); mm != nil {
		t.Fatalf("影视种应 nil: %+v", mm)
	}
	// 空 JSON
	if mm := musicMetaOfMeta(&model.TorrentMetadata{}); mm != nil {
		t.Fatalf("空 JSON 应 nil: %+v", mm)
	}
}

// TestGPWScreenshotGate §59.462 附九: 海豹截图三要件门——格式 PNG/图床
// 白名单/≥3 张（站规 2.2.1 硬性）。
func TestGPWScreenshotGate(t *testing.T) {
	// 不足 3 张
	if blocked, _ := gpwScreenshotGate([]string{"https://ptpimg.me/a.png"}); !blocked {
		t.Error("1 张应拦截（<3）")
	}
	// 3 张白名单域但非 PNG（ptpimg.me 在白名单）
	if blocked, _ := gpwScreenshotGate([]string{"https://ptpimg.me/1.jpg", "https://ptpimg.me/2.jpg", "https://ptpimg.me/3.jpg"}); !blocked {
		t.Error("非 PNG 应拦截")
	}
	// 3 张 PNG+白名单域
	if blocked, _ := gpwScreenshotGate([]string{"https://ptpimg.me/1.png", "https://ptpimg.me/2.png", "https://ptpimg.me/3.png"}); blocked {
		t.Error("3 张白名单 PNG 应放行")
	}
	// 非白名单域（x.com 不在八域内）
	if blocked, _ := gpwScreenshotGate([]string{"https://x.com/1.png", "https://x.com/2.png", "https://x.com/3.png"}); !blocked {
		t.Error("非白名单域应拦截")
	}
}

// TestGPWOnlyMovies §59.462 2d-①: 海豹纯电影站内容门——剧集拦截·电影放行。
func TestGPWOnlyMovies(t *testing.T) {
	// 电影（category movie）
	movieMeta := &model.TorrentMetadata{Category: "category.movie"}
	if blocked, _ := gpwOnlyMovies(movieMeta); blocked {
		t.Error("category.movie 应放行")
	}
	// 剧集
	tvMeta := &model.TorrentMetadata{Category: "category.tv_series"}
	if blocked, _ := gpwOnlyMovies(tvMeta); !blocked {
		t.Error("category.tv_series 应拦截")
	}
	// concert/short_film 也放行
	if blocked, _ := gpwOnlyMovies(&model.TorrentMetadata{Category: "category.concert"}); blocked {
		t.Error("category.concert 应放行")
	}
	// 空类别走 default=拦（白名单制·非电影系全拦）
	if blocked, _ := gpwOnlyMovies(&model.TorrentMetadata{}); !blocked {
		t.Error("空类别应拦（default 分支——非电影系全拦）")
	}
}

// TestHasMusic §59.490: 音乐种判定（fetchSingleTorrent 截图跳过门消费）。
func TestHasMusic(t *testing.T) {
	if !metadata.HasMusic(`{"music":{"artist":"X"}}`) {
		t.Error("含 music 应 true")
	}
	if metadata.HasMusic(`{"douban":{"title":"Y"}}`) {
		t.Error("无 music 应 false")
	}
	if metadata.HasMusic("") {
		t.Error("空应 false")
	}
}
