package download

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.508 刀1: 种子状态判定单点锚——完成优先于暂停。
// 案源：65 环境 qb「完成」态种子（stoppedUP=进度100%+暂停）被 IsPaused
// 优先分支恒判 paused → 转移查询只捞 completed → 71 颗永不进队。
func TestTorrentStatusOf(t *testing.T) {
	cases := []struct {
		name    string
		ti      model.TorrentInfo
		wantSt  string
		wantPro float64
	}{
		{
			name:    "完成+暂停（qb 完成态 stoppedUP）→ completed",
			ti:      model.TorrentInfo{IsFinished: true, IsPaused: true, Progress: 1.0},
			wantSt:  model.DownloadStatusCompleted,
			wantPro: 100,
		},
		{
			name:    "完成+活跃（qb 做种态 uploading）→ completed",
			ti:      model.TorrentInfo{IsFinished: true, IsPaused: false, Progress: 1.0},
			wantSt:  model.DownloadStatusCompleted,
			wantPro: 100,
		},
		{
			name:    "未完成+暂停（下载中暂停）→ paused",
			ti:      model.TorrentInfo{IsFinished: false, IsPaused: true, Progress: 0.5},
			wantSt:  model.DownloadStatusPaused,
			wantPro: 50,
		},
		{
			name:    "下载中 → downloading",
			ti:      model.TorrentInfo{IsFinished: false, IsPaused: false, Progress: 0.3},
			wantSt:  model.DownloadStatusDownloading,
			wantPro: 30,
		},
		{
			name:    "错误态 → error",
			ti:      model.TorrentInfo{IsFinished: false, IsPaused: false, Error: "xx", Progress: 0.1},
			wantSt:  model.DownloadStatusError,
			wantPro: 10,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, pro := torrentStatusOf(&c.ti)
			if st != c.wantSt {
				t.Errorf("status = %q, want %q", st, c.wantSt)
			}
			if pro != c.wantPro {
				t.Errorf("progress = %v, want %v", pro, c.wantPro)
			}
		})
	}
}
