package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestFormConfigDivergentKeysPaired §59.376: canonical 分叉族双键断言
// （防 §59.289→§59.376 同款第三犯）。
//
// 分叉根源：dict/type.json（发布消费 canonical）纪录片=category.documentary
// （单数）而 extended_standard_keys.json（采集归一）=category.documentaries
// （复数）——form_config 生成时从 extended 抄即得复数形态，发布消费列
// 精确匹配 miss。规则：form_config 资产里分叉族键必须双键并存。
// 已知分叉族：category.documentary/documentaries（dict canonical 基准
// 与 extended 值域分叉的对——新分叉族出现时补入 divergentFamilies）。
func TestFormConfigDivergentKeysPaired(t *testing.T) {
	divergentFamilies := [][]string{
		{"category.documentary", "category.documentaries"},
	}
	stdKeysArrRe := regexp.MustCompile(`"standard_keys":\s*\[(.*?)\]`)
	keyRe := regexp.MustCompile(`"([^"]+)"`)

	files, err := filepath.Glob("migration*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob migration*.go: %v files=%d", err, len(files))
	}
	// sites.json 是 form_config 的另一资产源（seed 链）
	sitesJSON := "../site/data/sites.json"
	if _, err := os.Stat(sitesJSON); err == nil {
		files = append(files, sitesJSON)
	}

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range stdKeysArrRe.FindAllStringSubmatch(string(src), -1) {
			keys := keyRe.FindAllString(m[1], -1)
			has := func(k string) bool {
				for _, x := range keys {
					if x == k {
						return true
					}
				}
				return false
			}
			for _, fam := range divergentFamilies {
				sing, plur := fam[0], fam[1]
				if has(plur) && !has(sing) {
					t.Errorf("%s: 分叉族键 %s 出现但缺 %s（发布消费 canonical）——须双键并存", f, plur, sing)
				}
			}
		}
	}
}
