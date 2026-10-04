// mteam-cat-fix §59.317 P1: 馒头存量分类刷正工具。
//
// 背景：v0.0.255-v0.0.1083 的 mteamCategoryMap 误用发布表单 ID 快照——馒头
// 2024-03 重建两级树后 ID 全错位（421=电影/BluRay 译 category.cartoon、
// 419=电影/HD 译 category.audiobook）。P0（v0.0.1084）已重建映射；本工具
// 刷正存量：不重 fetch（保 reviewed 状态），确定性逆映射。
//
// 逆映射依据（旧表两值唯一来源即馒头 map——normalize 族从不产出 cartoon）：
//   category.cartoon   ← 旧表 "421" → 权威树 电影/BluRay → category.movie
//   category.audiobook ← 旧表 "419" → 权威树 电影/HD    → category.movie
// category 列随 source_category 同步（InferCategory ①级短路：源站归一 movie
// 即 movie）。detail_source_json.type 同步更新。updated_at 不动（最小变更）。
//
// 用法：
//   mteam-cat-fix -db /path/pt-forward.db            # dry-run（计数+样本，零写入）
//   mteam-cat-fix -db /path/pt-forward.db -apply     # 实刷
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 旧值 → 新值（确定性逆映射，范围限定 site_name='馒头'）
var fixMap = map[string]string{
	"category.cartoon":   "category.movie", // 旧表 421=电影/BluRay
	"category.audiobook": "category.movie", // 旧表 419=电影/HD
}

type metaRow struct {
	ID              uint   `gorm:"primaryKey"`
	InfoHash        string `gorm:"column:info_hash"`
	SiteName        string `gorm:"column:site_name"`
	Title           string `gorm:"column:title"`
	SourceCategory  string `gorm:"column:source_category"`
	Category        string `gorm:"column:category"`
	DetailSourceJSON string `gorm:"column:detail_source_json"`
}

func (metaRow) TableName() string { return "torrent_metadata" }

func main() {
	dbPath := flag.String("db", "", "pt-forward.db 路径")
	apply := flag.Bool("apply", false, "实刷（默认 dry-run 零写入）")
	flag.Parse()
	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "用法: mteam-cat-fix -db /path/pt-forward.db [-apply]")
		os.Exit(1)
	}

	db, err := gorm.Open(sqlite.Open(*dbPath+"?_busy_timeout=10000"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "open db:", err)
		os.Exit(1)
	}

	// 范围：馒头站 × 旧表两值（其余站点/值不碰）
	var rows []metaRow
	if err := db.Where("site_name = ? AND source_category IN ?",
		"馒头", []string{"category.cartoon", "category.audiobook"}).
		Order("id").Find(&rows).Error; err != nil {
		fmt.Fprintln(os.Stderr, "query:", err)
		os.Exit(1)
	}

	fmt.Printf("目标行: %d（馒头 × cartoon/audiobook）\n", len(rows))
	shown := 0
	for _, r := range rows {
		if shown < 5 {
			fmt.Printf("  [%d] %s | %s | %s → %s\n", r.ID, r.InfoHash[:10], strHead(r.Title, 36), r.SourceCategory, "category.movie")
			shown++
		}
	}
	if len(rows) == 0 {
		fmt.Println("无需刷正（已是新口径或无馒头行）")
		return
	}
	if !*apply {
		fmt.Println("dry-run 结束（加 -apply 实刷）")
		return
	}

	start := time.Now()
	fixed, typeFixed := 0, 0
	for _, r := range rows {
		updates := map[string]interface{}{
			"source_category": "category.movie",
			"category":        "category.movie",
		}
		if r.DetailSourceJSON != "" {
			var m map[string]interface{}
			if json.Unmarshal([]byte(r.DetailSourceJSON), &m) == nil {
				if t, ok := m["type"].(string); ok && fixMap[t] != "" {
					m["type"] = fixMap[t]
					if b, err := json.Marshal(m); err == nil {
						updates["detail_source_json"] = string(b)
						typeFixed++
					}
				}
			}
		}
		if err := db.Model(&metaRow{}).Where("id = ?", r.ID).Updates(updates).Error; err != nil {
			fmt.Fprintf(os.Stderr, "  [id=%d] 更新失败: %v\n", r.ID, err)
			continue
		}
		fixed++
	}
	fmt.Printf("完成: %d/%d 行刷正（detail_source_json.type 同步 %d 行），耗时 %s\n",
		fixed, len(rows), typeFixed, time.Since(start).Round(time.Millisecond))
}

func strHead(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
