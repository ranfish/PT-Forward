package titleparser

import (
	_ "embed"
	"encoding/json"
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
