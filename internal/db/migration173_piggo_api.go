package db

// migration173 §59.540: 猪猪改版重接入——framework=piggo（令牌 API）+
// tls_pretend=chrome（CF 指纹墙·azuretls 通道）+ auth_type=apikey +
// base_url=https://api.piggo.me + fetch_rps 0.4（API 30/min×0.8 保守）+
// 官组映射四行（用户定案·catalog 吻合）+ RSS 消费列（hash_strategy=
// fake_from_id 触发侧载算真 hash——guid=torrent:{id} 非真 hash；id_strategy
// =path_segment 提 link=torrents/{id} 末段 tid；侧载走 adapter.DownloadTorrent
// （download-link 签发·TLSPretend 通道全链 azuretls）。
// 注意：is_target/publish_supported 保持关——form_config 值集刀后开闸。
import (
	"gorm.io/gorm"
)

func init() {
	RegisterMigration(173, "piggo_api_reentry", func(gormDB *gorm.DB) error {
		if err := gormDB.Exec(`UPDATE sites SET framework = 'piggo', auth_type = 'apikey',
			tls_pretend = 'chrome', base_url = 'https://api.piggo.me',
			fetch_rps = 0.4,
			hash_strategy = 'fake_from_id', id_strategy = 'path_segment',
			updated_at = datetime('now')
			WHERE domain = 'piggo.me'`).Error; err != nil {
			return err
		}
		// 官组映射四行（is_official=1——is_source 源站语义）
		for _, g := range []string{"PigoWeb", "PigoHD", "PigoNF", "PigoAD"} {
			if err := gormDB.Exec(`INSERT INTO release_group_mappings (group_name, domain, site_name, is_official, created_at, updated_at)
				SELECT ?, 'piggo.me', '二师兄', 1, datetime('now'), datetime('now')
				WHERE NOT EXISTS (SELECT 1 FROM release_group_mappings WHERE group_name = ? AND site_name = '二师兄')`,
				g, g).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
