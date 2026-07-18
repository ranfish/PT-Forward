// Package extract 提供 PT 站详情页 HTML 解析与字段提取能力。
//
// §56.7 决策：HTML 框架选用 goquery v1.12.0 + CSS selector 主路径，辅助用正则兜底。
// parser.go 仅提供统一入口，零封装负担（直接返回 *goquery.Document）。
package extract

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ErrEmptyHTML 输入 HTML 为空（调用方可用 errors.Is 判断并决定日志级别）。
var ErrEmptyHTML = errors.New("extract: empty html input")

// ParseHTML 从字符串解析 HTML，返回 goquery.Document。
// 空字符串返回 ErrEmptyHTML，调用方可据此降级日志级别（§56.7 P2-1）。
func ParseHTML(html string) (*goquery.Document, error) {
	if strings.TrimSpace(html) == "" {
		return nil, ErrEmptyHTML
	}
	return goquery.NewDocumentFromReader(strings.NewReader(html))
}

// ParseHTMLFromReader 从 io.Reader 解析（适用于流式抓取，避免内存拷贝）。
func ParseHTMLFromReader(r io.Reader) (*goquery.Document, error) {
	if r == nil {
		return nil, fmt.Errorf("extract: nil reader")
	}
	return goquery.NewDocumentFromReader(r)
}

// MustParseHTML 在解析失败时 panic，仅供测试/初始化阶段使用。
// 生产代码应使用 ParseHTML 并处理错误。
func MustParseHTML(html string) *goquery.Document {
	doc, err := ParseHTML(html)
	if err != nil {
		panic(fmt.Sprintf("extract: MustParseHTML failed: %v", err))
	}
	return doc
}
