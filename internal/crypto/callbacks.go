package crypto

import (
	"reflect"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func RegisterCallbacks(db *gorm.DB, enc *CredentialEncryptor, logger *zap.Logger) {
	_ = db.Callback().Create().Before("gorm:create").Register("crypto:encrypt", func(d *gorm.DB) {
		encryptFields(d, enc, logger)
	})
	_ = db.Callback().Update().Before("gorm:update").Register("crypto:encrypt", func(d *gorm.DB) {
		encryptFields(d, enc, logger)
	})
	_ = db.Callback().Query().After("gorm:query").Register("crypto:decrypt", func(d *gorm.DB) {
		decryptResults(d, enc, logger)
	})
}

func encryptFields(d *gorm.DB, enc *CredentialEncryptor, logger *zap.Logger) {
	if d.Statement == nil || d.Statement.Model == nil {
		return
	}
	val := reflect.ValueOf(d.Statement.Model)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}
	encryptStruct(val, enc, logger)
}

func encryptStruct(val reflect.Value, enc *CredentialEncryptor, logger *zap.Logger) {
	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Anonymous {
			if f := val.Field(i); f.Kind() == reflect.Struct {
				encryptStruct(f, enc, logger)
			}
			continue
		}
		if _, ok := field.Tag.Lookup("encrypted"); !ok {
			continue
		}
		fv := val.Field(i)
		if fv.Kind() != reflect.String {
			continue
		}
		plain := fv.String()
		if plain == "" {
			continue
		}
		if enc.IsEncrypted(plain) {
			continue
		}
		encValue, err := enc.Encrypt(plain)
		if err != nil {
			logger.Error("encrypt field failed", zap.String("field", field.Name), zap.Error(err))
			continue
		}
		fv.SetString(encValue)
	}
}

func decryptResults(d *gorm.DB, enc *CredentialEncryptor, logger *zap.Logger) {
	if d.Statement == nil || d.Statement.Dest == nil {
		return
	}
	dest := reflect.ValueOf(d.Statement.Dest)
	decryptValue(dest, enc, logger)
}

func decryptValue(val reflect.Value, enc *CredentialEncryptor, logger *zap.Logger) {
	switch val.Kind() {
	case reflect.Pointer:
		if val.IsNil() {
			return
		}
		decryptValue(val.Elem(), enc, logger)
	case reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			decryptValue(val.Index(i), enc, logger)
		}
	case reflect.Struct:
		if !hasEncryptedFields(val.Type()) {
			return
		}
		for i := 0; i < val.NumField(); i++ {
			fv := val.Field(i)
			if fv.Kind() == reflect.Pointer && !fv.IsNil() {
				decryptValue(fv.Elem(), enc, logger)
				continue
			}
			if fv.Kind() == reflect.Struct {
				ft := val.Type().Field(i)
				if ft.Anonymous || hasEncryptedFields(fv.Type()) {
					decryptValue(fv, enc, logger)
				}
				continue
			}
			if fv.Kind() != reflect.String {
				continue
			}
			ft := val.Type().Field(i)
			if _, ok := ft.Tag.Lookup("encrypted"); !ok {
				continue
			}
			encValue := fv.String()
			if encValue == "" || !enc.IsEncrypted(encValue) {
				continue
			}
			plain, err := enc.Decrypt(encValue)
			if err != nil {
				logger.Error("decrypt field failed", zap.String("field", ft.Name), zap.Error(err))
				continue
			}
			fv.SetString(plain)
		}
	}
}

func hasEncryptedFields(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if _, ok := f.Tag.Lookup("encrypted"); ok {
			return true
		}
		if f.Anonymous && f.Type.Kind() == reflect.Struct && hasEncryptedFields(f.Type) {
			return true
		}
	}
	return false
}

type encryptedFieldInfo struct {
	TableName string
	Columns   []string
}

func getEncryptedModels() []encryptedFieldInfo {
	return []encryptedFieldInfo{
		{TableName: "sites", Columns: []string{"passkey", "cookie", "api_key", "bearer_token", "auth_key", "auth_hash", "rss_key"}},
		{TableName: "clients", Columns: []string{"password"}},
		{TableName: "cookie_cloud_configs", Columns: []string{"password"}},
		{TableName: "iyuu_configs", Columns: []string{"token"}},
		{TableName: "notification_channels", Columns: []string{"config"}},
		{TableName: "site_config_overrides", Columns: []string{"field_value"}},
	}
}

func MigratePlaintext(db *gorm.DB, enc *CredentialEncryptor, logger *zap.Logger) error {
	for _, model := range getEncryptedModels() {
		for _, col := range model.Columns {
			var count int64
			err := db.Table(model.TableName).
				Where(col+" != '' AND "+col+" NOT LIKE ?", ciphertextPrefix+"%").
				Count(&count).Error
			if err != nil {
				if strings.Contains(err.Error(), "no such table") || strings.Contains(err.Error(), "doesn't exist") {
					continue
				}
				logger.Warn("migration check failed",
					zap.String("table", model.TableName),
					zap.String("column", col),
					zap.Error(err),
				)
				continue
			}
			if count == 0 {
				continue
			}
			logger.Info("migrating plaintext credentials",
				zap.String("table", model.TableName),
				zap.String("column", col),
				zap.Int64("count", count),
			)
			rows, err := db.Table(model.TableName).
				Select("id, "+col).
				Where(col+" != '' AND "+col+" NOT LIKE ?", ciphertextPrefix+"%").
				Rows()
			if err != nil {
				logger.Warn("migration query failed",
					zap.String("table", model.TableName),
					zap.String("column", col),
					zap.Error(err),
				)
				continue
			}
			type row struct {
				ID    int64
				Value string
			}
			var pending []row
			for rows.Next() {
				var id int64
				var plain string
				if err := rows.Scan(&id, &plain); err != nil {
					continue
				}
				pending = append(pending, row{ID: id, Value: plain})
			}
			_ = rows.Close()
			migrated := int64(0)
			for _, r := range pending {
				encValue, err := enc.Encrypt(r.Value)
				if err != nil {
					logger.Warn("encrypt failed during migration", zap.Error(err))
					continue
				}
				if err := db.Table(model.TableName).Where("id = ?", r.ID).Update(col, encValue).Error; err != nil {
					logger.Warn("migration update failed", zap.Int64("id", r.ID), zap.Error(err))
					continue
				}
				migrated++
			}
			logger.Info("migration completed",
				zap.String("table", model.TableName),
				zap.String("column", col),
				zap.Int64("migrated", migrated),
			)
		}
	}
	return nil
}
