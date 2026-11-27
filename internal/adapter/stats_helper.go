package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
)

// 14 刀3（v0.0.1659）: reSizeValue/sizeToBytes 手搓第二套已删除——
// 收敛至 extract.ParseSizeToBytes 全库唯一字节换算单点。

// flexFloat 14 刀1（v0.0.1659）: API 结构化响应专用浮点——string/number
// 双兼容（UNIT3D master UserResource 权威形态：ratio/seedbonus 均为
// string 数值 "50.00"·防御上游版本形态漂移时兼容裸数字）。
// 与 flexInt 的纪律分界（F-01 定性）：flexInt=页面文本抓取容错（解析
// 失败静默归零——「真 0」与「失败」不可区分但页面场景可接受）；
// flexFloat=API 契约场景显性化（解析失败返回 err→整体 unmarshal 失败
// →触发 FetchUserStats HTML 降级——失败不被伪装成合法零值）。
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("flexFloat: empty input")
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return fmt.Errorf("flexFloat: empty string")
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("flexFloat: %w", err)
		}
		*f = flexFloat(n)
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("flexFloat: %w", err)
	}
	*f = flexFloat(n)
	return nil
}

func parseSizeString(s string) int64 {
	s = strings.TrimSpace(s)
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	if s == "" {
		return 0
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f)
	}
	// 14 刀3 F-02（v0.0.1659）: 手搓 sizeToBytes 第二套删除——
	// 收敛至 extract.ParseSizeToBytes 全库唯一单点（其多小数点
	// 吸收〔"1.2.3"→1.23〕与裸 int64 上界漏网两缺陷根除·千分位
	// 逗号剥离能力保留·PB/PiB 能力对齐）。
	return extract.ParseSizeToBytes(s)
}

var reStripTags = regexp.MustCompile(`<[^>]+>`)

func stripHTMLTags(s string) string {
	return reStripTags.ReplaceAllString(s, "")
}

func cleanText(s string) string {
	s = stripHTMLTags(s)
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.TrimSpace(s)
}
