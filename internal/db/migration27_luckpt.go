package db

import (
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.147 切片 1 · migration 27: 幸运站首份 publish_form_config 落库。
//
// 数据源=site_field_mappings 幸运 77 条（切片 0 三方对照审计 17/17 全对——DB 播种数据为权威种子），
// label→standard_key 化合入（C2 三本通讯录并一本），HTML 实测字段名（§59.149：
// data-mode='4' 后缀 medium_sel[4] 等 + tags[4][] checkbox_span）。
// 条件标签 auto:false（§59.150）：英语（纯英语内容专用）/首发/原创/禁转（用户声明域禁自动勾）。
func seedLuckPublishFormConfig(gormDB *gorm.DB) error {
	var rows []model.SiteFieldMapping
	if err := gormDB.Where("site_name = ?", "幸运").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil // 无播种数据（空库环境）——跳过，切片 3 HTML 上传补
	}

	// field_type（site_field_mappings）→ 逻辑域（FormFields key）
	domainOf := map[string]string{
		"cat": model.FieldDomainType, "medium_sel": model.FieldDomainMedium,
		"codec_sel": model.FieldDomainCodec, "standard_sel": model.FieldDomainStandard,
		"audiocodec_sel": model.FieldDomainAudiocodec, "team_sel": model.FieldDomainTeam,
		"tags": model.FieldDomainTags,
	}
	// label → standard_key（dict canonical 权威对齐；空=待标注清单兜底——L3 新概念走切片 3 HTML diff 审计）
	skByLabel := map[string][]string{
		// type 域（10/10 全映射）
		"电影": {"category.movie"}, "电视剧": {"category.tv_series"}, "动画": {"category.animation"},
		"综艺": {"category.tv_shows"}, "纪录片": {"category.documentary"}, "音乐": {"category.music"},
		"体育": {"category.sports"}, "短剧": {"category.short_drama"}, "MV": {"category.mv"},
		"其他": {"category.other"},
		// medium 域（Track/CD/MiniBD/Other 待标注——medium.json 无对应 canonical）
		"Blu-ray": {"medium.bluray"}, "UHD Blu-ray": {"medium.uhd_bluray"}, "Remux": {"medium.remux"},
		"DVD": {"medium.dvd"}, "HDTV": {"medium.hdtv"}, "Encode": {"medium.encode"}, "WEB-DL": {"medium.webdl"},
		// codec 域（双键=vc 双形态 §59.151：站方一选项覆盖原生+重编码双语义）
		"H.264/AVC": {"video.h264", "video.x264"}, "H.265/HEVC": {"video.h265", "video.x265"},
		"AV1": {"video.av1"}, "MPEG-2": {"video.mpeg2"}, "VC-1": {"video.vc1"}, "MPEG-4/XviD": {"video.xvid"},
		// standard（分辨率）域
		"1080p/1080i": {"resolution.r1080p", "resolution.r1080i"}, "2K/1440p/1440i": {"resolution.r1440p"},
		"480p/480i": {"resolution.r480p"}, "4K/2160p/2160i": {"resolution.r2160p"},
		"720p/720i": {"resolution.r720p"}, "8K/4320p/4320i": {"resolution.r4320p"},
		// audiocodec 域
		"AAC": {"audio.aac"}, "APE": {"audio.ape"}, "DD/AC3": {"audio.dd"}, "DDP/E-AC3": {"audio.ddp"},
		"DTS": {"audio.dts"}, "DTS-HD MA": {"audio.dts_hd_ma"}, "DTS:X": {"audio.dts_x"},
		"FLAC": {"audio.flac"}, "LPCM": {"audio.lpcm"}, "MP3": {"audio.mp3"}, "TrueHD": {"audio.truehd"},
		"WAV": {"audio.wav"},
		// tags 域（§59.150 官方验证语义；首发/原创/禁转=用户声明域不映射+禁自动）
		"Dolby Vision": {"tag.dolby_vision"}, "HDR10": {"tag.hdr10"}, "HDR10+": {"tag.hdr10_plus"},
		"菁彩HDR": {"tag.hdr_vivid"}, "国语": {"tag.chinese_audio"}, "粤语": {"tag.cantonese_audio"},
		"中字": {"tag.chinese_subtitle"}, "DIY": {"tag.diy"}, "完结": {"tag.complete"},
		"连载": {"tag.ongoing"}, "合集": {"tag.collection"}, "大包": {"tag.big_pack"},
		"特效": {"tag.special_effects_subs"},
		// team 域全留空（R3-6 team 域待做——label/value 本身即数据）
	}
	// 条件标签（§59.150）：组合条件禁自动勾
	autoFalse := map[string]bool{"英语": true, "首发": true, "原创": true, "禁转": true}

	cfg := model.PublishFormConfig{
		Enabled:    true,
		Framework:  "np",
		PreAuditURL: "/api/auto-audit/pre-audit",
		FormFields: map[string]string{
			// 幸运 HTML 实测字段名（§59.149：data-mode='4' 下拉后缀 + checkbox_span）
			model.FieldDomainType:        "type",
			model.FieldDomainMedium:      "medium_sel[4]",
			model.FieldDomainCodec:       "codec_sel[4]",
			model.FieldDomainStandard:    "standard_sel[4]",
			model.FieldDomainAudiocodec:  "audiocodec_sel[4]",
			model.FieldDomainTeam:        "team_sel[4]",
			model.FieldDomainTags:        "tags[4][]",
			model.FieldDomainSmallDescr:  "small_descr",
			model.FieldDomainIMDBURL:     "url",
			model.FieldDomainDescription: "descr",
			model.FieldDomainTechInfo:    "technical_info",
		},
		ValueMappings: map[string][]model.FormValueMapping{},
	}
	for _, r := range rows {
		domain, ok := domainOf[r.FieldType]
		if !ok {
			continue
		}
		m := model.FormValueMapping{Label: r.SourceValue, Value: r.TargetValue}
		if sk := skByLabel[r.SourceValue]; len(sk) > 0 {
			m.StandardKeys = sk
		}
		if autoFalse[r.SourceValue] {
			f := false
			m.Auto = &f
		}
		cfg.ValueMappings[domain] = append(cfg.ValueMappings[domain], m)
	}

	var site model.Site
	if err := gormDB.Where("name = ?", "幸运").First(&site).Error; err != nil {
		return err // 站点行不存在——非幸运环境跳过
	}
	return gormDB.Model(&model.Site{}).Where("id = ?", site.ID).
		Update("publish_form_config", cfg.Serialize()).Error
}

func init() {
	RegisterMigration(27, "luckpt_publish_form_config", seedLuckPublishFormConfig)
}
