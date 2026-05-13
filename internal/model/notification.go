package model

import "time"

// §33.1.49 — NotificationChannel: 通知通道配置
type NotificationChannel struct {
	ID               uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Type             string    `json:"type" gorm:"size:32;not null"`
	Name             string    `json:"name" gorm:"size:100;not null"`
	Enabled          bool      `json:"enabled" gorm:"default:true"`
	Config           string    `json:"config" gorm:"type:text" encrypted:"true"`
	Events           string    `json:"events" gorm:"type:text"`
	MaxErrorsPerHour int       `json:"max_errors_per_hour" gorm:"default:100"`
	Overrides        string    `json:"overrides" gorm:"type:text"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	Healthy             bool   `json:"healthy" gorm:"default:true"`
	ConsecutiveFailures int    `json:"consecutive_failures" gorm:"default:0"`
	TimeoutMs           int    `json:"timeout_ms" gorm:"default:10000"`
	QuietHoursStart     string `json:"quiet_hours_start" gorm:"size:5;default:''"`
	QuietHoursEnd       string `json:"quiet_hours_end" gorm:"size:5;default:''"`
	FailoverGroupID     string `json:"failover_group_id" gorm:"size:50;default:''"`
	MinPriority         int    `json:"min_priority" gorm:"default:3"`
	DigestTemplate      string `json:"digest_template" gorm:"type:text"`
	MessageTemplate     string `json:"message_template" gorm:"type:text"`
	DigestIntervalMin   int    `json:"digest_interval_min" gorm:"default:60"`
}

func (NotificationChannel) TableName() string { return "notification_channels" }

// §33.1.50 — NotificationHistory: 通知投递记录
type NotificationHistory struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ChannelID uint      `json:"channel_id" gorm:"not null;index"`
	Event     string    `json:"event" gorm:"size:64;not null"`
	Level     string    `json:"level" gorm:"size:16;not null"`
	Title     string    `json:"title" gorm:"size:256"`
	Body      string    `json:"body" gorm:"type:text"`
	Success   bool      `json:"success" gorm:"default:true"`
	ErrorMsg  string    `json:"error_msg" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}

func (NotificationHistory) TableName() string { return "notification_history" }

// §33.1.17 — FormattedMessage: 通知消息标准载体
type FormattedMessage struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Level   string `json:"level"`
}
