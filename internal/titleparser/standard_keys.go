package titleparser

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed data/standard_keys.json
var standardKeysJSON []byte

// StandardKeyData embed JSON 中的标准键定义。
type StandardKeyData struct {
	Category    string   `json:"category"`
	Key         string   `json:"key"`
	Code        string   `json:"code"`
	Aliases     []string `json:"aliases"`
	IsProtected bool     `json:"is_protected"`

	// FoldTo §59.268: 编码器实现归标准折叠目标（x264→video.h264）——
	// 站方表单普遍只有标准名选项而压制标题 token 是编码器名，映射 miss 时
	// 折叠兜底（executor 消费）。数据化扩展：新编码族加词条零代码。
	FoldTo string `json:"fold_to,omitempty"`
}

var (
	foldOnce  sync.Once
	foldIndex map[string]string // "category|code" → fold_to
)

func buildFoldIndex() {
	foldIndex = map[string]string{}
	keys, err := LoadStandardKeys()
	if err != nil {
		return
	}
	for _, k := range keys {
		if k.FoldTo != "" {
			foldIndex[k.Category+"|"+k.Code] = k.FoldTo
		}
	}
}

// FoldStandardKey §59.268: 域内标准键折叠（x264→video.h264——编码器实现归标准）。
// 无折叠词条返回原值；域泛化（category 参数——未来 medium 3D→Blu-ray 等同机制）。
func FoldStandardKey(category, code string) string {
	if code == "" {
		return ""
	}
	foldOnce.Do(buildFoldIndex)
	if to, ok := foldIndex[category+"|"+code]; ok && to != "" {
		return to
	}
	return code
}

// LoadStandardKeys 解析 embed JSON 为标准键列表（纯函数，不依赖 DB）。
func LoadStandardKeys() ([]StandardKeyData, error) {
	var data struct {
		Keys []StandardKeyData `json:"keys"`
	}
	if err := json.Unmarshal(standardKeysJSON, &data); err != nil {
		return nil, err
	}
	return data.Keys, nil
}
