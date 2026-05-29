package db

import (
	"encoding/json"
	"reflect"
	"time"

	"gorm.io/gorm"
)

func ForceCreate(db *gorm.DB, obj interface{}) error {
	return forceCreateDB(db, obj)
}

func ForceCreateTx(tx *gorm.DB, obj interface{}) error {
	return forceCreateDB(tx, obj)
}

func forceCreateDB(d *gorm.DB, obj interface{}) error {
	stmt := &gorm.Statement{DB: d}
	if err := stmt.Parse(obj); err != nil {
		return err
	}

	rv := reflect.ValueOf(obj)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}

	now := time.Now()
	s := stmt.Schema
	colMap := make(map[string]interface{}, len(s.Fields))
	var pkField *reflect.Value
	for _, f := range s.Fields {
		if f.PrimaryKey {
			fv := rv.FieldByName(f.Name)
			pkField = &fv
			continue
		}
		fv := rv.FieldByName(f.Name)
		if !fv.IsValid() {
			continue
		}
		if f.Name == "CreatedAt" || f.Name == "UpdatedAt" {
			val := fv.Interface()
			if t, ok := val.(time.Time); ok && t.IsZero() {
				colMap[f.DBName] = now
				fv.Set(reflect.ValueOf(now))
				continue
			}
		}
		val := fv.Interface()
		if f.TagSettings["SERIALIZER"] == "json" {
			b, err := json.Marshal(val)
			if err != nil {
				return err
			}
			colMap[f.DBName] = string(b)
		} else {
			colMap[f.DBName] = val
		}
	}

	tx := d.Model(obj)
	if err := tx.Create(colMap).Error; err != nil {
		return err
	}

	if pkField != nil && pkField.CanSet() && pkField.Uint() == 0 {
		var lastID uint
		if err := d.Table(s.Table).Select("id").Order("id DESC").Limit(1).Scan(&lastID).Error; err == nil && lastID > 0 {
			pkField.SetUint(uint64(lastID))
		}
	}

	return nil
}
