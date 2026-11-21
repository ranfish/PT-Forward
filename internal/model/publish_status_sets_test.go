package model

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPublishStatusSetsLocked §59.546 终态清单锚：内容锁定+互斥面。
//
// 新增发布终态时必须显式更新此锚——红灯会强制你检查两清单的语义归属：
//   - PublishedListStatuses（列表·记录在案口径·含 existing）
//   - PublishSkipStatuses（跳过/拦截·发布事实口径·不含 existing）
//
// 互斥面断言的用户定案（2026-11-22 Hatfields 案）：
// 「跳过=替用户做决定」——existing=文案信任（无 tid·未推种）·用户还
// 没行使过推种权·系统无权预先替用户放弃 → 永不进 Skip 清单。
func TestPublishStatusSetsLocked(t *testing.T) {
	wantList := []string{"pushed", "pushed_existing", "uploaded", "uploaded_existing", "duplicate", "existing"}
	wantSkip := []string{"pushed", "pushed_existing", "uploaded", "uploaded_existing", "duplicate"}

	if got := strings.Join(PublishedListStatuses, ","); got != strings.Join(wantList, ",") {
		t.Fatalf("PublishedListStatuses 内容漂移:\n  got  %s\n  want %s\n新增终态须先登记语义归属再改锚", got, strings.Join(wantList, ","))
	}
	if got := strings.Join(PublishSkipStatuses, ","); got != strings.Join(wantSkip, ",") {
		t.Fatalf("PublishSkipStatuses 内容漂移:\n  got  %s\n  want %s\n新增终态须先登记语义归属再改锚", got, strings.Join(wantSkip, ","))
	}

	// 互斥面：existing 只进 List（用户定案·防未来误统一）
	skipSet := map[string]bool{}
	for _, s := range PublishSkipStatuses {
		skipSet[s] = true
	}
	if skipSet["existing"] {
		t.Fatal("existing 不得进 PublishSkipStatuses——保留用户重试/推种选择权（2026-11-22 用户定案·见 model/publish.go 哲学注释）")
	}
	// Skip ⊆ List（拦截口径是记录口径的子集——结构性约束）
	listSet := map[string]bool{}
	for _, s := range PublishedListStatuses {
		listSet[s] = true
	}
	for s := range skipSet {
		if !listSet[s] {
			t.Fatalf("PublishSkipStatuses 的 %q 不在 PublishedListStatuses 中——拦截口径应是记录口径子集", s)
		}
	}
}

// TestNoBarePublishStatusList §59.546 源码扫描锚：internal/ 源码里禁止
// 手写发布终态裸清单（指纹="pushed" 与 "pushed_existing" 相邻的字面量
// 清单形态）——只许出现在常量定义处（model/publish.go）。
//
// 防「第四处手写」：新增消费点必须引用 model 常量。rss/repository.go 的
// {"pushed","expired",...} 是 RSS 处理状态异族（pushed 后非 pushed_existing
// 形态）天然不命中指纹。
func TestNoBarePublishStatusList(t *testing.T) {
	re := regexp.MustCompile(`"pushed",\s*"pushed_existing"`)
	root := ".."
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && (info.Name() == "testdata" || strings.HasPrefix(info.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+"/"))
		for _, loc := range re.FindAllStringIndex(string(raw), -1) {
			line := strings.Count(string(raw)[:loc[0]], "\n") + 1
			t.Errorf("%s:%d 出现发布终态裸清单——须引用 model.PublishedListStatuses / model.PublishSkipStatuses（§59.546 单点登记处）", rel, line)
		}
		return nil
	})
}
