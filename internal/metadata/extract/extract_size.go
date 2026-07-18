package extract

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var sizeValueRe = regexp.MustCompile(`(?i)([\d.,]+)\s*(TiB|GiB|MiB|KiB|TB|GB|MB|KB|B)`)

// extractSize 从详情页解析种子大小（字节）。
// 主路径：goquery 找含"大小"的 dt/dd 或 th/td 行。
// Fallback：正则扫全文取第一个匹配。
func (p *PublicExtractor) extractSize(doc *goquery.Document, htmlStr string) int64 {
	if s := findSizeInRows(doc); s != "" {
		if v := parseSizeToBytes(s); v > 0 {
			return v
		}
	}
	// Fallback：正则
	if m := sizeValueRe.FindStringSubmatch(htmlStr); len(m) > 2 {
		return parseSizeToBytes(m[1] + " " + m[2])
	}
	return 0
}

// findSizeInRows 扫描 dt/dd 或 th/td 含 "大小"/"Size" 的行。
func findSizeInRows(doc *goquery.Document) string {
	var found string
	keywords := []string{"大小", "Size", "文件大小", "檔案大小"}
	doc.Find("dt").Each(func(_ int, dt *goquery.Selection) {
		if found != "" {
			return
		}
		text := dt.Text()
		for _, kw := range keywords {
			if strings.Contains(text, kw) || containsInsensitive(text, kw) {
				dd := dt.NextFiltered("dd")
				if dd.Length() > 0 {
					found = strings.TrimSpace(dd.Text())
					return
				}
			}
		}
	})
	if found != "" {
		return found
	}
	doc.Find("th").Each(func(_ int, th *goquery.Selection) {
		if found != "" {
			return
		}
		text := th.Text()
		for _, kw := range keywords {
			if strings.Contains(text, kw) || containsInsensitive(text, kw) {
				td := th.NextFiltered("td")
				if td.Length() > 0 {
					found = strings.TrimSpace(td.Text())
					return
				}
			}
		}
	})
	return found
}

// parseSizeToBytes 解析 "1.5 GB" / "800 MB" 等为字节数。
// 支持 B/KB/MB/GB/TB 和 KiB/MiB/GiB/TiB（合并 PT-Forward 旧 parseSizeStr + parseSizeString）。
func parseSizeToBytes(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	m := sizeValueRe.FindStringSubmatch(s)
	if len(m) < 3 {
		return 0
	}
	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	switch strings.ToUpper(m[2]) {
	case "TB", "TIB":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	case "GB", "GIB":
		return int64(val * 1024 * 1024 * 1024)
	case "MB", "MIB":
		return int64(val * 1024 * 1024)
	case "KB", "KIB":
		return int64(val * 1024)
	default:
		return int64(val)
	}
}
