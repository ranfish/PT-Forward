package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ranfish/pt-forward/internal/site"
)

func NewDebugSearchHandler(siteProvider *site.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		siteName := r.URL.Query().Get("site")
		keyword := r.URL.Query().Get("keyword")
		if siteName == "" || keyword == "" {
			Error(w, http.StatusBadRequest, 40001, "site and keyword are required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		config, err := siteProvider.GetSiteConfig(ctx, siteName)
		if err != nil || config == nil {
			Error(w, http.StatusNotFound, 40400, "site config not found: "+siteName)
			return
		}

		adapter, err := siteProvider.GetAdapter(ctx, siteName)
		if err != nil || adapter == nil {
			Error(w, http.StatusNotFound, 40400, "adapter not found: "+siteName)
			return
		}

		results, err := adapter.SearchTorrents(ctx, config, keyword, nil)
		if err != nil {
			Error(w, http.StatusInternalServerError, 50000, "search error: "+err.Error())
			return
		}

		type item struct {
			TorrentID string `json:"torrent_id"`
			Title     string `json:"title"`
			Size      int64  `json:"size"`
			SizeDisp  string `json:"size_display"`
			Seeders   int    `json:"seeders"`
		}

		items := make([]item, 0, len(results))
		for _, res := range results {
			disp := ""
			if res.Size > 0 {
				gb := float64(res.Size) / 1073741824.0
				if gb >= 1 {
					disp = fmt.Sprintf("%.2f GiB", gb)
				} else {
					disp = fmt.Sprintf("%.2f MiB", gb*1024)
				}
			}
			items = append(items, item{
				TorrentID: res.TorrentID,
				Title:     res.Title,
				Size:      res.Size,
				SizeDisp:  disp,
				Seeders:   res.Seeders,
			})
		}

		Success(w, map[string]interface{}{
			"site":    siteName,
			"keyword": keyword,
			"count":   len(items),
			"results": items,
		})
	}
}
