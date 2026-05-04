package model

import "time"

// §33.1.87 — OperationAuditLog: 运行时操作审计日志（Sprint 105）
type OperationAuditLog struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`

	Actor      string `json:"actor" gorm:"size:50;index"`
	Module     string `json:"module" gorm:"size:30;index"`
	Action     string `json:"action" gorm:"size:50;not null"`
	TargetType string `json:"target_type" gorm:"size:30;index"`
	TargetID   string `json:"target_id" gorm:"size:100;index"`

	Detail string `json:"detail" gorm:"type:text"`
	Result string `json:"result" gorm:"size:20"`
}

func (OperationAuditLog) TableName() string { return "operation_audit_logs" }

// §33.1.89 — SchemaMigration: 版本间数据迁移框架（Sprint 105）
type SchemaMigration struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Version   int       `json:"version" gorm:"uniqueIndex;not null"`
	AppliedAt time.Time `json:"applied_at"`
}

func (SchemaMigration) TableName() string { return "schema_migrations" }
