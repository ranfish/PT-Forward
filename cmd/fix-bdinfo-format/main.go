// cmd/fix-bdinfo-format §59.319 附四：BDInfo 存量刷正（fix* 临时工具，用后即删）。
// ①bd_info 列全量过 FormatBDInfoReport（幂等——已格式化文本无变化）
// ②source_media_info 实为 BDInfo 文本且 bd_info 空（fetch 落库缺口时代数据）
//   → 迁入 bd_info 列 + 清 source_media_info（归位语义）
// 用法：fix-bdinfo-format <db-path> [--dry]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dbPath := flag.String("db", "", "sqlite db path")
	dry := flag.Bool("dry", false, "dry run（只统计不写）")
	flag.Parse()
	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "usage: fix-bdinfo-format -db <path> [--dry]")
		os.Exit(1)
	}
	db, err := gorm.Open(sqlite.Open(*dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}

	var rows []model.TorrentMetadata
	if err := db.Where("bd_info != '' OR source_media_info != ''").Find(&rows).Error; err != nil {
		fmt.Fprintln(os.Stderr, "query:", err)
		os.Exit(1)
	}
	formatted, migrated, skipped := 0, 0, 0
	for i := range rows {
		r := &rows[i]
		updates := map[string]interface{}{}
		if r.BDInfo != "" {
			f := titleparser.FormatBDInfoReport(r.BDInfo)
			if f != r.BDInfo {
				updates["bd_info"] = f
				formatted++
			} else {
				skipped++
			}
		} else if r.SourceMediaInfo != "" && extract.IsLikelyBDInfoText(r.SourceMediaInfo) {
			updates["bd_info"] = titleparser.FormatBDInfoReport(r.SourceMediaInfo)
			updates["source_media_info"] = ""
			migrated++
		}
		if len(updates) == 0 || *dry {
			continue
		}
		if err := db.Model(&model.TorrentMetadata{}).Where("id = ?", r.ID).Updates(updates).Error; err != nil {
			fmt.Fprintf(os.Stderr, "update id=%d: %v\n", r.ID, err)
			os.Exit(1)
		}
	}
	fmt.Printf("total=%d formatted=%d migrated=%d already-ok=%d dry=%v\n",
		len(rows), formatted, migrated, skipped, *dry)
}
