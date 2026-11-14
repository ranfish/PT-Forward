package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration161 §59.520: 织梦（zmpt.cc）tags/audiocodec 补 keys——四颗 UHD
// DoVi HDR 种被站方审核拒「检测到杜比视界，请补充杜比标签」+「音频类: Other」
// （Who Am I/When Harry Met Sally 的 DTS-HD MA 落 Other 错选）。
//
// 实时复拉 upload.php 全量对账（2026-11-14·登录态 57KB 逐字）：
// ① tags 16 项值集零漂移·「杜比(14)/HDR(7)」词条在场但零键（适配期只配了
//    6 键——杜比/HDR 被标「✅ 交叉验证」却漏接=站方校验标签与 MI 一致性·
//    我们漏投必拒）
// ② audiocodec_sel 站方实拉 10 项确认（无 DTS-HD MA/TrueHD/LPCM 词条——
//    §59.466「DTS-HD MA→Other=音乐向站语义正确」定案作废：织梦是综合站
//    （影视区）·审核脚本音频识别表含 DTS/TrueHD/LPCM/DDP=影视语义）
//
// 语义定案（用户确认 2026-11-14）：
//   tags:     杜比(14) ← tag.dolby_vision·HDR(7) ← tag.hdr10
//             （HDR10+/HLG 织梦无词条不配·miss 蒸发；无损/短剧/汉化/原创/
//              驻站/纯享版零键保持——无对应键或不该自动投）
//   audiocodec: DTS(3) 补 [dts_hd_ma, dts_hd_hr, dts_x]（DTS 家族归并·用户实证）
//             ·WAV(10) 补 [lpcm]（无压缩语义精准·LPCM 2.0 同族预防）
//   挂账：TrueHD/Atmos 归并方向（10 项无贴切·等重发后站方反馈）
//
// 幂等：条目已含目标键则跳过。全量 map 拉出改回（保 form_config 全字段）。
func init() {
	RegisterMigration(161, "zmpt_tag_audio_keys", func(gormDB *gorm.DB) error {
		type patch struct {
			domain string
			label  string
			keys   []string
		}
		patches := []patch{
			{"tags", "杜比", []string{"tag.dolby_vision"}},
			{"tags", "HDR", []string{"tag.hdr10"}},
			{"audiocodec", "DTS", []string{"audio.dts", "audio.dts_hd_ma", "audio.dts_hd_hr", "audio.dts_x"}},
			{"audiocodec", "WAV", []string{"audio.wav", "audio.lpcm"}},
		}
		var cfgStr string
		if err := gormDB.Raw(
			`SELECT publish_form_config FROM sites WHERE domain = 'zmpt.cc' AND publish_form_config IS NOT NULL`,
		).Scan(&cfgStr).Error; err != nil {
			return err
		}
		if cfgStr == "" {
			return nil // 站点不存在或无配置——幂等跳过
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(cfgStr), &cfg); err != nil {
			return nil // 配置形态异常——不动
		}
		vm, ok := cfg["value_mappings"].(map[string]interface{})
		if !ok {
			return nil
		}
		changed := false
		for _, p := range patches {
			list, ok := vm[p.domain].([]interface{})
			if !ok {
				continue
			}
			for _, item := range list {
				m, ok := item.(map[string]interface{})
				if !ok || m["label"] != p.label {
					continue
				}
				keys, _ := m["standard_keys"].([]interface{})
				merged := make([]interface{}, 0, len(keys)+len(p.keys))
				existing := map[string]bool{}
				for _, k := range keys {
					if s, ok := k.(string); ok {
						existing[s] = true
						merged = append(merged, s)
					}
				}
				for _, k := range p.keys {
					if !existing[k] {
						merged = append(merged, k)
						changed = true
					}
				}
				if len(merged) > 0 {
					m["standard_keys"] = merged
				}
			}
		}
		if !changed {
			return nil
		}
		out, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return gormDB.Exec(
			`UPDATE sites SET publish_form_config = ?, updated_at = datetime('now') WHERE domain = 'zmpt.cc'`,
			string(out)).Error
	})
}
