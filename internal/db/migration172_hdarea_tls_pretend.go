package db

// §59.540 好大 TLS 指纹迁移（用户定案 b）：hdarea tls_pretend='chrome'——
// azuretls chromeTransport（§59.334 附二基建）243 现场实证 3 连发 200（§59.1487
// Go 标准栈 30s 超时挂起现场根治·0.1s 级秒回）。
// 消费面零改动：sites.tls_pretend 列既有——adapter factory+RSS engine
// （§59.333 通道）全链自动生效。
import (
	"gorm.io/gorm"
)

func init() {
	RegisterMigration(172, "hdarea_tls_pretend", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET tls_pretend = 'chrome', updated_at = datetime('now')
			WHERE domain = 'hdarea.club' AND (tls_pretend IS NULL OR tls_pretend = '')`).Error
	})
}
