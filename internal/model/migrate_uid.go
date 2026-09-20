package model

// migrateLegacyClientUID §59.251/v0.0.992 旧库升级兼容层。
//
// 背景：§59.251 恒定 ID 重构把 17 表的 client_id（下载器名字 string）改为
// client_uid（数字 ID）。旧库不删 data 直接升级时 gorm AutoMigrate 撞两层墙：
//  1. NOT NULL 无 default 的 ALTER（v0.0.991 已修——tag 补 default:0）
//  2. 旧行 client_uid 全 0 → 唯一索引 UNIQUE 冲突（v0.0.992 本层修）
//
// 本层在 AutoMigrate 逐模型迁移前执行，逐表幂等：
//  a. 旧列（client_id）存在且新列不存在 → 手动 ALTER ADD 新列 DEFAULT 0
//     （先于 gorm，绕过其建索引时序）
//  b. 名字回填：client_uid = (SELECT id FROM clients WHERE name = client_id)，
//     回填不上（孤儿名/已删下载器）→ 保持 0
//  c. 唯一索引表去重：uid=0 的行按新唯一键分组仅留 MAX(id)——保证 gorm
//     建索引成功；语义：孤儿行本就无宿主，留一行供人工处置
//  d. 复合角色列（source_client_id/transfer_client_id）同法回填
//  e. 数组列（rss transfer_client_uids/candidate_clients、reseed_tasks.client_ids
//     名字串）不做 SQL 回填——Go 层逐行映射（clients 名字→id）
//
// 语义边界：本层=升级路径不 crash + 尽力保留数据；回填后的库仍可能含
// uid=0 残行（孤儿引用），业务正确性终态仍建议删库重配（§59.251 删库路线）。

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// legacyUIDTable 单表的迁移描述
type legacyUIDTable struct {
	table    string   // 表名
	oldCols  []string // 待回填的旧列（名字形态）
	dedupKey []string // 唯一索引列组（空=无唯一索引，免去重）
}

var legacyUIDTables = []legacyUIDTable{
	{table: "torrent_snapshots", oldCols: []string{"client_id"}, dedupKey: []string{"hash", "client_uid"}},
	{table: "seeding_torrent_records", oldCols: []string{"client_id"}, dedupKey: []string{"client_uid", "info_hash"}},
	{table: "seeding_client_configs", oldCols: []string{"client_id"}, dedupKey: []string{"client_uid"}},
	{table: "seeding_client_states", oldCols: []string{"client_id"}, dedupKey: []string{"client_uid"}},
	{table: "cluster_screenshot_cache", oldCols: []string{"client_id"}, dedupKey: []string{"client_uid", "save_path", "name"}},
	{table: "client_publish_targets", oldCols: []string{"client_id"}, dedupKey: []string{"client_uid", "site_name"}},
	{table: "reseed_matches", oldCols: []string{"client_id"}, dedupKey: []string{"client_uid", "source_site", "source_torrent_id", "target_site", "target_torrent_id"}},
	{table: "traffic_stats_hourly", oldCols: []string{"client_id"}, dedupKey: []string{"client_uid", "hour"}},
	{table: "orphan_scan_configs", oldCols: []string{"client_id"}, dedupKey: []string{"client_uid", "scan_path"}},
	{table: "publish_group_members", oldCols: []string{"client_id"}}, // 旧唯一键不含 client 列——免去重
	{table: "free_wait_entries", oldCols: []string{"client_id"}},     // idx_sitewait 不含 client——免去重
	{table: "torrent_traffic", oldCols: []string{"client_id"}},
	{table: "downloader_speed_snapshots", oldCols: []string{"client_id"}},
	{table: "scoring_logs", oldCols: []string{"client_id"}},
	{table: "download_tasks", oldCols: []string{"client_id", "transfer_client_id"}},
	{table: "publish_candidates", oldCols: []string{"client_id", "source_client_id"}},
	{table: "rss_subscriptions", oldCols: []string{"client_id"}},
}

// 旧列名 → 新列名（角色映射）
func uidNewCol(old string) string {
	switch old {
	case "client_id":
		return "client_uid"
	case "source_client_id":
		return "source_client_uid"
	case "transfer_client_id":
		return "transfer_client_uid"
	}
	return ""
}

func migrateLegacyClientUID(db *gorm.DB) {
	migrateLegacyUIDNotNull(db) // 必须最先：解除旧列 NOT NULL（表同构重建）——否则新代码 INSERT 被旧列拦截
	for _, t := range legacyUIDTables {
		migrateLegacyTable(db, t)
	}
	migrateLegacyRssArrays(db)
	migrateLegacyReseedTasks(db)
	migrateLegacyUIDIndexes(db) // 必须在回填/去重后：DROP 旧列组索引，gorm AutoMigrate 重建正确列组
}

// migrateLegacyUIDNotNull 第〇步：旧 client_id 列 NOT NULL 解除。
// 旧列自带 NOT NULL 约束（新代码 INSERT 不写旧列 → 全部拦截，243 第四层实证：
// 快照 upsert rows:0）。SQLite 不支持 ALTER 改约束——同构表重建：原 DDL 仅去掉
// client_id 列的 NOT NULL，列序不变，INSERT SELECT * 同构拷贝（零列映射风险）。
// DROP TABLE 连带删全部索引，gorm AutoMigrate 按新模型重建（半迁移态已建的新列组
// 索引同删同建，幂等）。torrent_events 的自引用外键不受影响（本层不触该表）。
func migrateLegacyUIDNotNull(db *gorm.DB) {
	tables := []string{
		"torrent_snapshots", "seeding_torrent_records", "seeding_client_configs", "seeding_client_states",
		"reseed_matches", "free_wait_entries", "download_tasks", "orphan_scan_configs",
		"cluster_screenshot_cache", "client_publish_targets", "scoring_logs",
	}
	for _, t := range tables {
		if !legacyTableExists(db, t) || !legacyHasColumn(db, t, "client_id") {
			continue
		}
		if !legacyColumnNotNull(db, t, "client_id") {
			continue // 已解除（幂等）
		}
		var ddl string
		if err := db.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name = ?", t).Scan(&ddl).Error; err != nil || ddl == "" {
			continue
		}
		// 去掉 client_id 列定义中的 NOT NULL（保留 DEFAULT 等其余约束）
		re := regexp.MustCompile("(`client_id`[^,)]*?)\\s+NOT NULL")
		newDDL := re.ReplaceAllString(ddl, "$1")
		if newDDL == ddl {
			continue
		}
		tmp := t + "__uidmig"
		if err := db.Exec("DROP TABLE IF EXISTS " + tmp).Error; err != nil {
			continue
		}
		if err := db.Exec(strings.Replace(newDDL, "CREATE TABLE `"+t+"`", "CREATE TABLE `"+tmp+"`", 1)).Error; err != nil {
			db.Logger.Error(db.Statement.Context, "legacy uid migrate: recreate table failed: %v table=%s", err, t)
			continue
		}
		if err := db.Exec("INSERT INTO " + tmp + " SELECT * FROM " + t).Error; err != nil {
			db.Logger.Error(db.Statement.Context, "legacy uid migrate: copy rows failed: %v table=%s", err, t)
			db.Exec("DROP TABLE IF EXISTS " + tmp)
			continue
		}
		if err := db.Exec("DROP TABLE " + t).Error; err != nil {
			continue
		}
		if err := db.Exec("ALTER TABLE " + tmp + " RENAME TO " + t).Error; err != nil {
			db.Logger.Error(db.Statement.Context, "legacy uid migrate: rename failed: %v table=%s", err, t)
			continue
		}
	}
}

func legacyColumnNotNull(db *gorm.DB, table, col string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ? AND \"notnull\" = 1", table, col).Scan(&n)
	return n > 0
}

// migrateLegacyUIDIndexes 第五步：旧列组索引替换。
// 旧库唯一/普通索引列组用旧列名（如 idx_snapshot_hash_client ON (hash, client_id)），而 gorm
// AutoMigrate 见同名索引存在即跳过——导致 (hash, client_uid) 索引缺失，upsert
// ON CONFLICT (hash, client_uid) 全静默失败（243 第三层实证）。DROP 后 gorm 按新
// 模型重建（数据已回填+去重，唯一索引重建安全）。client_path_mappings 的
// source_client_id/reseed_client_id 列名未变——排除。
func migrateLegacyUIDIndexes(db *gorm.DB) {
	type idxRow struct {
		Name string
		Tbl  string
	}
	var idxs []idxRow
	db.Raw("SELECT name, tbl_name FROM sqlite_master WHERE type = 'index' AND sql IS NOT NULL AND (sql LIKE '%client_id%' OR sql LIKE '%client_id %') AND sql NOT LIKE '%client_uid%' AND tbl_name != 'client_path_mappings'").Scan(&idxs)
	for _, ix := range idxs {
		if err := db.Exec("DROP INDEX IF EXISTS " + ix.Name).Error; err != nil {
			db.Logger.Error(db.Statement.Context, "legacy uid migrate: drop old index failed: %v idx=%s", err, ix.Name)
		}
	}
}

func migrateLegacyTable(db *gorm.DB, t legacyUIDTable) {
	if !legacyTableExists(db, t.table) {
		return // 新装（表由 gorm 建）或更老版本无此表
	}
	for _, old := range t.oldCols {
		newCol := uidNewCol(old)
		if newCol == "" {
			continue
		}
		hasOld := legacyHasColumn(db, t.table, old)
		hasNew := legacyHasColumn(db, t.table, newCol)
		if !hasOld && !hasNew {
			continue // 既无旧列也无新列：非本迁移域（gorm 建新表）
		}
		if hasOld && !hasNew {
			// 手动加列（先于 gorm 的索引创建时序）
			if err := db.Exec("ALTER TABLE " + t.table + " ADD COLUMN " + newCol + " integer NOT NULL DEFAULT 0").Error; err != nil {
				db.Logger.Error(db.Statement.Context, "legacy uid migrate: add column failed: %v table=%s col=%s", err, t.table, newCol)
				continue
			}
		}
		if !hasOld {
			continue // 新库（gorm 已建新列）或已迁移完
		}
		// 名字回填（幂等：仅 uid=0 且旧名非空的行）
		if err := db.Exec("UPDATE " + t.table + " SET " + newCol + " = COALESCE((SELECT id FROM clients WHERE clients.name = " + t.table + "." + old + "), 0) WHERE " + newCol + " = 0 AND " + old + " IS NOT NULL AND " + old + " != ''").Error; err != nil {
			db.Logger.Error(db.Statement.Context, "legacy uid migrate: backfill failed: %v table=%s", err, t.table)
		}
	}
	// 唯一索引表去重（回填后仍 uid=0 的孤儿行按唯一键留一行）
	if len(t.dedupKey) > 0 {
		keyJoin := strings.Join(t.dedupKey, ", ")
		sql := "DELETE FROM " + t.table + " WHERE client_uid = 0 AND id NOT IN (SELECT MAX(id) FROM " + t.table + " WHERE client_uid = 0 GROUP BY " + keyJoin + ")"
		if err := db.Exec(sql).Error; err != nil {
			db.Logger.Error(db.Statement.Context, "legacy uid migrate: dedup failed: %v table=%s", err, t.table)
		}
	}
}

// migrateLegacyRssArrays rss_subscriptions 数组列（json 名字数组→id 数组）Go 层映射
func migrateLegacyRssArrays(db *gorm.DB) {
	if !legacyTableExists(db, "rss_subscriptions") || !legacyHasColumn(db, "rss_subscriptions", "client_id") {
		return
	}
	type row struct {
		ID                      uint
		ClientID                string
		ReseedClientIDs         string // 旧 auto_reseed 列族（json 名字数组）
		TransferClientIDs       string
		CandidateClients        string
	}
	var rows []row
	if err := db.Raw("SELECT id, COALESCE(client_id,''), COALESCE(reseed_client_ids,''), COALESCE(transfer_client_ids,''), COALESCE(candidate_clients,'') FROM rss_subscriptions WHERE client_uid = 0 OR (reseed_client_ids IS NOT NULL AND reseed_client_ids != '') OR (candidate_clients IS NOT NULL AND candidate_clients != '')").Scan(&rows).Error; err != nil {
		return
	}
	if len(rows) == 0 {
		return
	}
	nameToID := legacyClientNameMap(db)
	for _, r := range rows {
		uid := nameToID[r.ClientID]
		updates := map[string]interface{}{}
		if uid != 0 {
			updates["client_uid"] = uid
		}
		if tc := legacyMapJSONArray(r.TransferClientIDs, nameToID); tc != nil {
			updates["transfer_client_uids"] = tc
		}
		if cc := legacyMapJSONArray(r.CandidateClients, nameToID); cc != nil {
			updates["candidate_clients"] = cc
		}
		if len(updates) > 0 {
			db.Model(&RSSSubscription{}).Where("id = ?", r.ID).Updates(updates)
		}
	}
}

// migrateLegacyReseedTasks reseed_tasks.client_ids（逗号分隔名字串→数字串）Go 层映射
func migrateLegacyReseedTasks(db *gorm.DB) {
	if !legacyTableExists(db, "reseed_tasks") || !legacyHasColumn(db, "reseed_tasks", "client_ids") {
		return
	}
	var hasNonNumeric int64
	db.Raw("SELECT COUNT(*) FROM reseed_tasks WHERE client_ids LIKE '%[^0-9,]%' ESCAPE '\\' OR client_ids LIKE '%[A-Za-z]%'").Scan(&hasNonNumeric)
	if hasNonNumeric == 0 {
		return // 全数字串（已是新形态）
	}
	type row struct {
		ID        uint
		ClientIDs string
	}
	var rows []row
	if err := db.Raw("SELECT id, COALESCE(client_ids,'') FROM reseed_tasks").Scan(&rows).Error; err != nil {
		return
	}
	nameToID := legacyClientNameMap(db)
	for _, r := range rows {
		parts := strings.Split(r.ClientIDs, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if id := nameToID[p]; id != 0 {
				out = append(out, fmtUint(id))
			}
		}
		db.Exec("UPDATE reseed_tasks SET client_ids = ? WHERE id = ?", strings.Join(out, ","), r.ID)
	}
}

func legacyClientNameMap(db *gorm.DB) map[string]uint {
	type cRow struct {
		ID   uint
		Name string
	}
	var cs []cRow
	db.Raw("SELECT id, name FROM clients").Scan(&cs)
	m := make(map[string]uint, len(cs))
	for _, c := range cs {
		m[c.Name] = c.ID
	}
	return m
}

// legacyMapJSONArray json 名字数组 → json id 数组（映射不上返回 nil=不更新）
func legacyMapJSONArray(raw string, nameToID map[string]uint) []byte {
	if raw == "" || raw == "null" || raw == "[]" {
		return nil
	}
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "[") {
		return nil // 非 json 数组形态（历史脏值）——不动
	}
	var names []string
	if err := jsonUnmarshal([]byte(trimmed), &names); err != nil {
		return nil
	}
	ids := make([]uint, 0, len(names))
	for _, n := range names {
		if id := nameToID[n]; id != 0 {
			ids = append(ids, id)
		}
	}
	return jsonMarshal(ids)
}

func jsonUnmarshal(b []byte, v interface{}) error { return json.Unmarshal(b, v) }
func jsonMarshal(v interface{}) []byte            { b, _ := json.Marshal(v); return b }
func fmtUint(u uint) string                       { return strconv.FormatUint(uint64(u), 10) }

func legacyTableExists(db *gorm.DB, name string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n)
	return n > 0
}

func legacyHasColumn(db *gorm.DB, table, col string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, col).Scan(&n)
	return n > 0
}
