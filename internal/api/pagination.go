package api

import (
	"net/http"
	"strconv"
)

type PaginatedResult struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

// parsePagination §59.552: 标准分页解析——page 默认 1（非法/<=0 回落·
// >10000 截断）；size 默认 20（有效域 1..200·非法/越界回落默认）。
//
// size 参数键优先级（§59.552 定案·测试锁定）：size > pageSize > page_size，
// 按序取第一个有效值——兼容前端三键名混用实态（seeding 用 size·reseed 用
// pageSize·publish 用 page_size）；多键同传以最优先的有效值为准。
func parsePagination(r *http.Request) (page, size int) {
	return parsePaginationWith(r, 20, 200, 10000)
}

// parsePaginationWith §59.552: 特殊默认值端点变体（如默认 size 50）——
// 边界契约与 parsePagination 一致，仅默认值/上限可调。为个别有意偏离
// 标准默认的端点保留（不强行复用标准默认）。
func parsePaginationWith(r *http.Request, defaultSize, maxSize, maxPage int) (page, size int) {
	page = 1
	size = defaultSize
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if maxPage > 0 && page > maxPage {
		page = maxPage
	}
	for _, key := range []string{"size", "pageSize", "page_size"} {
		s := r.URL.Query().Get(key)
		if s == "" {
			continue
		}
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= maxSize {
			size = v
			break
		}
	}
	return
}

func offset(page, size int) int {
	return (page - 1) * size
}
