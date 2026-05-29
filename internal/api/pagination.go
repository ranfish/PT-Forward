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

func parsePagination(r *http.Request) (page, size int) {
	page = 1
	size = 20
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if page > 10000 {
		page = 10000
	}
	if s := r.URL.Query().Get("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 200 {
			size = v
		}
	}
	if s := r.URL.Query().Get("pageSize"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 200 {
			size = v
		}
	}
	return
}

func offset(page, size int) int {
	return (page - 1) * size
}
