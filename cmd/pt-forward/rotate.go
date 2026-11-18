package main

// §仓库安全-3（R-02 定案）第四刀：应急密钥轮换 CLI（pt-forward --rotate-key）。
//
// 定案背景：加密密钥只存 config.yaml、永不进 DB、不做自动轮换；唯一保留
// 本应急工具——万一密钥再次泄漏时手动执行。触发路径不在 initEncryption
// 之后（不注册 GORM 加密回调），全程 raw 读写避免旧密钥回调二次加密。
//
// 流程：备份 → 逐表逐列（crypto.EncryptedColumnIndex 单源）旧解密→新加密
// → 抽验 → 写 config（替换式）→ 清 DB 密钥残留行。
// crash 窗口：重加密提交后、config 写入前崩溃 = DB 新密钥/config 旧密钥，
// 恢复路径 = 恢复两个备份文件（流程日志明示位置）。

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/config"
	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/setting"
)

func runRotateKey(cfg *config.Config, configPath string, db *gorm.DB, log *zap.Logger) error {
	oldKey := cfg.Security.EncryptionKey
	if oldKey == "" {
		return errors.New("config 无 encryption_key——先正常启动一次完成密钥落位（config.yaml）再执行轮换")
	}
	oldEnc, err := crypto.NewCredentialEncryptor(oldKey)
	if err != nil {
		return fmt.Errorf("旧密钥无效: %w", err)
	}
	newKey, err := crypto.GenerateRandomKey()
	if err != nil {
		return fmt.Errorf("生成新密钥: %w", err)
	}
	newEnc, err := crypto.NewCredentialEncryptor(newKey)
	if err != nil {
		return fmt.Errorf("新密钥初始化: %w", err)
	}

	// ① 备份（crash 窗口恢复路径）
	dbBak, cfgBak, err := backupForRotate(db, cfg, configPath)
	if err != nil {
		return fmt.Errorf("轮换前备份失败（已中止·数据未动）: %w", err)
	}
	log.Info("轮换前备份完成", zap.String("db", dbBak), zap.String("config", cfgBak))

	// ② 逐表逐列重加密（raw 读写·不经回调）
	idx := crypto.EncryptedColumnIndex()
	tables := make([]string, 0, len(idx))
	for t := range idx {
		tables = append(tables, t)
	}
	sort.Strings(tables)

	var totalRows, totalCols int
	for _, table := range tables {
		cols := make([]string, 0, len(idx[table]))
		for c := range idx[table] {
			cols = append(cols, c)
		}
		sort.Strings(cols)

		rows, err := db.Raw(fmt.Sprintf("SELECT id, %s FROM %s", strings.Join(cols, ","), table)).Rows()
		if err != nil {
			return fmt.Errorf("读表 %s 失败: %w", table, err)
		}
		type pending struct {
			id   int64
			sets []string
			args []interface{}
		}
		var batch []pending
		for rows.Next() {
			var id int64
			vals := make([]sql.NullString, len(cols))
			dest := make([]interface{}, 0, len(cols)+1)
			dest = append(dest, &id)
			for i := range vals {
				dest = append(dest, &vals[i])
			}
			if err := rows.Scan(dest...); err != nil {
				rows.Close()
				return fmt.Errorf("扫描 %s 行失败: %w", table, err)
			}
			p := pending{id: id}
			for i, c := range cols {
				v := vals[i]
				if !v.Valid || v.String == "" || !oldEnc.IsEncrypted(v.String) {
					continue // 空值/历史明文行跳过（明文保持明文，下次写入时回调自然加密）
				}
				plain, err := oldEnc.Decrypt(v.String)
				if err != nil {
					rows.Close()
					return fmt.Errorf("%s.%s id=%d 旧密钥解密失败（密钥不匹配?）: %w", table, c, id, err)
				}
				re, err := newEnc.Encrypt(plain)
				if err != nil {
					rows.Close()
					return fmt.Errorf("%s.%s id=%d 新密钥加密失败: %w", table, c, id, err)
				}
				p.sets = append(p.sets, fmt.Sprintf("%s = ?", c))
				p.args = append(p.args, re)
			}
			if len(p.sets) > 0 {
				batch = append(batch, p)
			}
		}
		rows.Close()
		for _, p := range batch {
			args := append(p.args, p.id)
			if err := db.Exec(fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", table, strings.Join(p.sets, ", ")), args...).Error; err != nil {
				return fmt.Errorf("更新 %s id=%d 失败: %w", table, p.id, err)
			}
			totalRows++
			totalCols += len(p.sets)
		}
	}
	log.Info("重加密完成", zap.Int("行", totalRows), zap.Int("列值", totalCols))

	// ③ 抽验：任取 sites.passkey 一条用新密钥解密（有数据时）
	var sample sql.NullString
	_ = db.Raw("SELECT passkey FROM sites WHERE passkey != '' LIMIT 1").Scan(&sample).Error
	if sample.Valid && sample.String != "" {
		if _, err := newEnc.Decrypt(sample.String); err != nil {
			return fmt.Errorf("抽验失败（新密钥解不开重加密值·请恢复备份）: %w", err)
		}
		log.Info("抽验通过（新密钥可解密重加密数据）")
	}

	// ④ 写 config（替换式）
	if configPath == "" {
		return errors.New("configPath 为空——DB 已重加密但新密钥未落 config；请手动将以下密钥写入 config.yaml 后重启：\n" + newKey)
	}
	if err := replaceEncryptionKeyInConfig(configPath, newKey); err != nil {
		return fmt.Errorf("DB 已重加密但写 config 失败（请手动写入以下密钥后重启·或恢复备份重试）: %w\n密钥: %s", err, newKey)
	}

	// ⑤ 清 DB 密钥残留行
	_ = setting.NewRepository(db).Delete(context.Background(), "encryption_key")

	log.Info("✓ 密钥轮换完成",
		zap.String("config", configPath),
		zap.String("db备份", dbBak),
		zap.String("config备份", cfgBak))
	log.Info("确认服务正常后可删除备份；异常时恢复两个备份文件即回滚到轮换前状态")
	return nil
}

// backupForRotate 备份 DB 文件（WAL checkpoint 后 copy）与 config。
func backupForRotate(db *gorm.DB, cfg *config.Config, configPath string) (dbBak, cfgBak string, err error) {
	ts := time.Now().Format("20060102-150405")
	if cfg.Database.Driver == "sqlite" {
		db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		src := cfg.Database.SQLitePath
		dbBak = src + ".rotate-bak-" + ts
		if err := copyFile(src, dbBak); err != nil {
			return "", "", fmt.Errorf("备份 DB: %w", err)
		}
		// WAL/SHM 侧文件（若存在也一并备份）
		for _, ext := range []string{"-wal", "-shm"} {
			if _, e := os.Stat(src + ext); e == nil {
				_ = copyFile(src+ext, dbBak+ext)
			}
		}
	} else {
		return "", "", errors.New("mysql 驱动请先用 mysqldump 手动备份后再执行轮换")
	}
	if configPath != "" {
		if _, e := os.Stat(configPath); e == nil {
			cfgBak = configPath + ".rotate-bak-" + ts
			if err := copyFile(configPath, cfgBak); err != nil {
				return "", "", fmt.Errorf("备份 config: %w", err)
			}
		}
	}
	return dbBak, cfgBak, nil
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0600)
}

// replaceEncryptionKeyInConfig 替换式写入（writeEncryptionKeyToConfig 是追加式——
// 轮换/迁移场景 config 可能已有 security 段，追加会产生重复键）。
// 形态矩阵：段在+键在→替换 · 段在+键缺→段内插入 · 段缺→追加新段。
func replaceEncryptionKeyInConfig(path, newKey string) error {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return os.WriteFile(path, []byte("security:\n  encryption_key: "+newKey+"\n"), 0640)
	}
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	keyLine := "  encryption_key: " + newKey

	secStart := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "security:" {
			secStart = i
			break
		}
	}
	if secStart < 0 {
		out := strings.TrimRight(string(content), "\n") + "\n\nsecurity:\n  encryption_key: " + newKey + "\n"
		return os.WriteFile(path, []byte(out), 0640)
	}
	// 段范围 = secStart+1 .. secEnd（下一个顶级键行或文件尾；顶级=非空非缩进非注释）
	secEnd := len(lines)
	for i := secStart + 1; i < len(lines); i++ {
		l := lines[i]
		if l == "" || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t") || strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		secEnd = i
		break
	}
	for i := secStart + 1; i < secEnd; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "encryption_key:") {
			indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
			lines[i] = indent + "encryption_key: " + newKey
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0640)
		}
	}
	// 段内无键：插入段首之后
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:secStart+1]...)
	out = append(out, keyLine)
	out = append(out, lines[secStart+1:]...)
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0640)
}
