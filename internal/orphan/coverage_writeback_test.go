package orphan

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/reseed"
	"go.uber.org/zap"
)

// 最小合法 bencode：d4:infod4:name3:abc6:lengthi123e12:piece lengthi16384eee
var minimalTorrent = []byte("d4:infod4:name3:abc6:lengthi123e12:piece lengthi16384eee")

type fakeCoverageWriter struct {
	records []*model.SiteCoverageCache
	err     error
}

func (f *fakeCoverageWriter) UpsertCoverage(ctx context.Context, r *model.SiteCoverageCache) error {
	if f.err != nil {
		return f.err
	}
	f.records = append(f.records, r)
	return nil
}

// §59.196: 恢复链下载成功后 tid 回写——coverage 记录字段完整性。
func TestWriteDownloadCoverage(t *testing.T) {
	fw := &fakeCoverageWriter{}
	r := NewRecovery(nil, nil, nil, zap.NewNop())
	r.SetCoverageService(fw)

	r.writeDownloadCoverage(context.Background(), minimalTorrent, "不可说", "288084")

	if len(fw.records) != 1 {
		t.Fatalf("expected 1 coverage record, got %d", len(fw.records))
	}
	rec := fw.records[0]
	if rec.InfoHash == "" {
		t.Error("expected non-empty info_hash")
	}
	if rec.TorrentID != "288084" || rec.SiteName != "不可说" {
		t.Errorf("tid/site mismatch: %s/%s", rec.TorrentID, rec.SiteName)
	}
	if rec.Status != model.CoverageConfirmedHas {
		t.Errorf("status = %s, want confirmed_has", rec.Status)
	}
	if rec.Source != model.CoverageSourceDownload {
		t.Errorf("source = %s, want download", rec.Source)
	}
	if !rec.ExpiresAt.After(time.Now()) {
		t.Error("expected future ExpiresAt")
	}
}

// §59.196: 防御分支——tid 空 / 种子数据不可解析 / 写失败：静默降级不 panic。
func TestWriteDownloadCoverage_Defensive(t *testing.T) {
	fw := &fakeCoverageWriter{}
	r := NewRecovery(nil, nil, nil, zap.NewNop())
	r.SetCoverageService(fw)

	r.writeDownloadCoverage(context.Background(), minimalTorrent, "不可说", "")
	r.writeDownloadCoverage(context.Background(), []byte("not-bencode"), "不可说", "1")
	if len(fw.records) != 0 {
		t.Fatalf("expected 0 records on empty tid / bad data, got %d", len(fw.records))
	}

	fw.err = context.Canceled
	r.writeDownloadCoverage(context.Background(), minimalTorrent, "不可说", "288084") // 写失败仅记日志
}

// §59.196: 未注入 coverage 服务（测试/降级场景）时静默跳过。
func TestWriteDownloadCoverage_NilService(t *testing.T) {
	r := NewRecovery(nil, nil, nil, zap.NewNop())
	r.writeDownloadCoverage(context.Background(), minimalTorrent, "不可说", "288084")
}

// StripYearToken §59.198: 年份剥离（站方年份笔误家族降级轮的词法基础）。
func TestStripYearToken(t *testing.T) {
	cases := []struct{ in, want string }{
		{"拾芳 2019 1080p", "拾芳 1080p"},
		{"巴士底日 2016", "巴士底日"},
		{"生死格斗 2006", "生死格斗"},
		{"Movie 1994 720p", "Movie 720p"},
		{"Movie 720p", "Movie 720p"},
		{"2001太空漫游 1968", "2001太空漫游"}, // CJK 词内 2001 不剥（非独立 token）
	}
	for _, c := range cases {
		if got := reseed.StripYearToken(c.in); got != c.want {
			t.Errorf("StripYearToken(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
