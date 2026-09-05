package db

import (
	"encoding/json"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// §59.168 · migration 37: PTGen 数据资产列 + 存量回填。
// 4 新列：chinese_title/english_title/genre/ptgen_meta。
// 回填：遍历 torrent_metadata，description 含◎行批量提取填新列（818 条憨憨种秒级）。
func addPTGenAssets(gormDB *gorm.DB) error {
	type colCheck struct{ Count int }
	var c colCheck
	gormDB.Raw("SELECT COUNT(*) as count FROM pragma_table_info('torrent_metadata') WHERE name='chinese_title'").Scan(&c)
	if c.Count == 0 {
		if err := gormDB.Exec(`ALTER TABLE torrent_metadata ADD COLUMN chinese_title VARCHAR(200) DEFAULT ''`).Error; err != nil {
			return err
		}
	}
	gormDB.Raw("SELECT COUNT(*) as count FROM pragma_table_info('torrent_metadata') WHERE name='english_title'").Scan(&c)
	if c.Count == 0 {
		if err := gormDB.Exec(`ALTER TABLE torrent_metadata ADD COLUMN english_title VARCHAR(500) DEFAULT ''`).Error; err != nil {
			return err
		}
	}
	gormDB.Raw("SELECT COUNT(*) as count FROM pragma_table_info('torrent_metadata') WHERE name='genre'").Scan(&c)
	if c.Count == 0 {
		if err := gormDB.Exec(`ALTER TABLE torrent_metadata ADD COLUMN genre VARCHAR(200) DEFAULT ''`).Error; err != nil {
			return err
		}
	}
	gormDB.Raw("SELECT COUNT(*) as count FROM pragma_table_info('torrent_metadata') WHERE name='ptgen_meta'").Scan(&c)
	if c.Count == 0 {
		if err := gormDB.Exec(`ALTER TABLE torrent_metadata ADD COLUMN ptgen_meta TEXT DEFAULT ''`).Error; err != nil {
			return err
		}
	}
	// §59.168 ⑨ 存量回填：遍历含◎行的记录批量提取填新列
	// 用 Go 正则（与 SetPTGenFields 同款逻辑——SQL 不可行因◎行全角空格+正则复杂）
	rows := []struct {
		ID          uint   `gorm:"column:id"`
		Description string `gorm:"column:description"`
	}{}
	gormDB.Raw("SELECT id, description FROM torrent_metadata WHERE description LIKE '%◎%' AND (chinese_title = '' OR genre = '')").Scan(&rows)
	for _, r := range rows {
		ct := extractPTGenValue(r.Description, "◎片　　名")
		et := extractPTGenEnglish(r.Description)
		gr := extractPTGenGenre(r.Description)
		pm := extractPTGenMeta(r.Description)
		gormDB.Exec("UPDATE torrent_metadata SET chinese_title=?, english_title=?, genre=?, ptgen_meta=? WHERE id=?",
			ct, et, gr, pm, r.ID)
	}
	return nil
}

// migration 内嵌提取函数（不依赖 metadata 包——migration 独立性）
func extractPTGenValue(desc, prefix string) string {
	m := regexp.MustCompile(regexp.QuoteMeta(prefix) + `[\s　]+(.+)`).FindStringSubmatch(desc)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractPTGenEnglish(desc string) string {
	m := regexp.MustCompile(`◎译　　名\s+([A-Za-z0-9][A-Za-z0-9\s\.\-']*(?::\s+[A-Za-z0-9][A-Za-z0-9\s\.\-']*)*)`).FindStringSubmatch(desc)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractPTGenGenre(desc string) string {
	raw := extractPTGenValue(desc, "◎类　　别")
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "/")
	out := []string{}
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return ""
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func extractPTGenMeta(desc string) string {
	meta := map[string]interface{}{}
	if v := extractPTGenValue(desc, "◎产　　地"); v != "" { meta["country"] = v }
	if v := extractPTGenValue(desc, "◎导　　演"); v != "" { meta["director"] = strings.Split(v, "/") }
	if v := extractPTGenValue(desc, "◎主　　演"); v != "" { meta["actor"] = strings.Split(v, "/") }
	if len(meta) == 0 { return "" }
	b, _ := json.Marshal(meta)
	return string(b)
}

func init() {
	// §59.168 回滚——PTGen 方案重新设计后重新启用
	// RegisterMigration(37, "add_ptgen_assets", addPTGenAssets)
}
