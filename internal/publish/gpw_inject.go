package publish

import (
	"context"
	"net/url"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
)

// ═══════════════════════════════════════════════════════════════
// §59.462 刀⑤: 海豹（GazellePW）发布翻译层——meta → gpw_ ExtraFields
// 通道（adapter 消费）。挂载模式：幸运英语/肉丝/野马同款站点特化。
// ═══════════════════════════════════════════════════════════════

// gpwOnlyMovies §59.462 2d-①: 纯电影站内容拦截——禁剧集/综艺/体育/
// MV/教程/动漫剧（站规 1.2 全族）。动画电影放行（S/E 特判——§59.424
// 禁分集的 SeasonEpisode 判定复用：animation 无 S/E=电影形态）。
func gpwOnlyMovies(meta *model.TorrentMetadata) (bool, string) {
	cat := meta.Category
	switch cat {
	case "category.movie", "category.concert", "category.short_film":
		return false, ""
	case "category.animation":
		// 动画电影 vs 动画剧：S/E 信号判（§59.424 同源——tp SeasonEpisode
		// 非空=剧集形态拦截；空=电影形态放行）
		if metaHasSeasonEpisode(meta) {
			return true, "海豹纯电影站禁剧集（animation 含 S/E 信号=动画剧）"
		}
		return false, ""
	default:
		// tv_series/variety/sports/mv/music/education 等全拦
		return true, fmt.Sprintf("海豹纯电影站禁此类型（%s）——仅收电影/短片/现场演出/电影集", cat)
	}
}

// metaHasSeasonEpisode S/E 信号探（标题/副标题——§59.424 词形族）。
// 简化实现：tp.SeasonEpisode 非空即剧集（调用方传 tp）。
func metaHasSeasonEpisode(meta *model.TorrentMetadata) bool {
	return titleparser.ParseTitleTech(meta.Title).SeasonEpisode != ""
}

// gpwPublishInject §59.462 刀⑤ 主注入——挂 executor jobs 段后 DryRun
// 检查点前（§59.453 挂载教训）。三个输出通道：
//   - form（GPW 六裸名槽——source/codec/resolution/container/processing
//     经通用 jobs 链填入，此处只做 processing 特化覆盖+碟规格）
//   - ExtraFields（gpw_ 前缀——建组八字段+subtitle_type+EditionInfo+8
//     特性 checkbox）
//   - ExtraArtists（剧组四件套）+SubtitleLangs
func gpwPublishInject(cfg *model.PublishFormConfig, form map[string]string,
	meta *model.TorrentMetadata, tp titleparser.TechProfile, size int64,
	extra map[string]string, artists *[]model.PublishArtist, subLangs *[]string) {

	// ===== 建组八字段（§59.462 附七: PTGen JSON 直读 + 派生列优先 + tp 兜底
	// 三层——DryRun 实证 meta 派生列填充率低（314 颗 movie 仅 2 颗
	// english_title 非空）而 PTGen JSON 全在（§59.236 X 重构四键单源））=====
	ptgen := gpwParsePTGen(meta.PTGenSourceJSON)
	extra["gpw_name"] = gpwFirst(meta.EnglishTitle, ptgen.ForeignTitle, tp.MainTitle)
	extra["gpw_subname"] = gpwFirst(meta.ChineseTitle, ptgen.ChineseTitle)
	if tp.Year != "" {
		extra["gpw_year"] = tp.Year
	} else if n, nErr := ptgen.Year.Int64(); nErr == nil && n > 0 {
		extra["gpw_year"] = fmt.Sprintf("%d", n)
	}
	// releasetype: concert→5 现场演出；其余→1 长片（短片简化定案）
	extra["gpw_releasetype"] = "1"
	if meta.Category == "category.concert" {
		extra["gpw_releasetype"] = "5"
	}
	// tags: genre 中文→英文小写（派生列优先——string 逗号分隔；列空读
	// PTGen genre 数组）
	genres := gpwGenreTags(strings.Split(meta.Genre, ","))
	if len(genres) == 0 {
		genres = gpwGenreTags(ptgen.Genre)
	}
	if len(genres) > 0 {
		extra["gpw_tags"] = strings.Join(genres, ",")
	}
	// imdb: URL 或 tt 均可（站方正则提取）——列/PTGen URL/PTGen id 三层
	extra["gpw_imdb"] = gpwFirst(meta.IMDbURL, ptgen.IMDbURL, ptgen.IMDbID)
	// desc: PTGen 影片简介（列优先——列含簇合并产物）
	extra["gpw_desc"] = gpwFirst(meta.Description, ptgen.Introduction)
	// image: 封面（原始 dbposter——executor 层 Rehost 转投白名单域）
	if meta.Poster != "" {
		extra["gpw_image"] = meta.Poster
	}

	// ===== 剧组四件套 =====
	if artists != nil {
		*artists = gpwBuildArtists(meta.PTGenSourceJSON)
	}

	// ===== 版本层特化 =====
	// processing 特化：原盘→Untouched+碟规格（体积判）
	if procField := cfg.FormFields["processing"]; procField != "" {
		if isOriginalDisc(tp) {
			// 原盘族→Untouched+processing_other 碟规格
			form[procField] = "Untouched"
			if discSpec := gpwDiscSpec(size); discSpec != "" {
				if poField := cfg.FormFields["processing_other"]; poField != "" {
					form[poField] = discSpec
				}
			}
		}
		// Remux/Encode/WEB-DL 走通用链 standard_keys 直配（remux→Remux/
		// encode→Encode/webdl→WEB+processing=---——form_config 已映射）
	}

	// ===== subtitle_type+subtitles[] =====
	// 1=混合（MI 中文字幕轨）/2=硬字幕/3=无字幕（简化：有中字标签→1）
	subType := "3" // 无字幕默认
	hasZhSub := strings.Contains(meta.Tags, "chinese_subtitle") ||
		strings.Contains(meta.Tags, "中字") ||
		strings.Contains(meta.Tags, "中文")
	if hasZhSub {
		subType = "1"
		extra["gpw_subtitle_type"] = subType
		if subLangs != nil {
			*subLangs = append(*subLangs, "chinese_simplified")
		}
	} else {
		extra["gpw_subtitle_type"] = subType
	}

	// ===== EditionInfo→remaster_title 英文键 =====
	if ek := gpwEditionKey(meta.EditionInfo); ek != "" {
		extra["gpw_remaster_title"] = ek
	}

	// ===== 特性 checkbox 族（8 个——MI/标签判定）=====
	gpwCheckboxFill(meta, tp, extra)
}

// gpwGenreTags 中文 genre→英文小写（站内存储形态——§59.462 B 项）。
var gpwGenreMap = map[string]string{
	"动作": "action", "冒险": "adventure", "动画": "animation", "艺术": "art",
	"亚洲": "asian", "传记": "biography", "喜剧": "comedy", "犯罪": "crime",
	"邪典": "cult", "纪录片": "documentary", "剧情": "drama", "实验": "experimental",
	"家庭": "family", "奇幻": "fantasy", "黑色电影": "film_noir", "历史": "history",
	"恐怖": "horror", "同性": "lgbt", "武侠": "martial_arts", "音乐": "music",
	"音乐剧": "musical", "悬疑": "mystery", "演出": "performance", "政治": "political",
	"爱情": "romance", "科幻": "sci_fi", "短片": "short", "默片": "silent",
	"体育": "sport", "惊悚": "thriller", "战争": "war", "西部": "western",
}

func gpwGenreTags(genres []string) []string {
	var out []string
	for _, g := range genres {
		if en, ok := gpwGenreMap[strings.TrimSpace(g)]; ok {
			out = append(out, en)
		}
	}
	return out
}

// gpwDiscSpec 体积→碟规格（原盘 processing_other——§59.462 体积判）。
// §59.462 附八: size<=0（快照 miss/Scan 失败）或 <20GB（真原盘实体最小
// BD25≈23GB——体积不足=误判原盘）返回空串不投——回归审核 P1：
// 旧实现 size=0 时 gib=0≤25.5 错投 BD25。
func gpwDiscSpec(size int64) string {
	gib := float64(size) / (1 << 30)
	if gib < 20 {
		return ""
	}
	switch {
	case gib <= 25.5:
		return "BD25"
	case gib <= 51:
		return "BD50"
	case gib <= 67:
		return "BD66"
	case gib <= 101:
		return "BD100"
	default:
		return "BD100" // >100GB 超规格边缘——取最近档
	}
}

// gpwEditionKey EditionInfo→站方英文键（EditionInfo 类全集——确定性词
// 才投：拿不准不投=原始版本，站方无校验拒收风险时默认不填）。
var gpwEditionMap = map[string]string{
	"director's cut": "director_s_cut",
	"directors cut":  "director_s_cut",
	"导演剪辑版":       "director_s_cut",
	"extended":       "extended_edition",
	"加长版":          "extended_edition",
	"uncut":          "uncut",
	"未删减":          "uncut",
	"unrated":        "unrated",
	"未分级":          "unrated",
	"remaster":       "remaster",
	"4k remaster":    "4k_remaster",
	"4k重制":          "4k_remaster",
	"criterion":      "the_criterion_collection",
	"cc":             "the_criterion_collection",
	"标准收藏":         "the_criterion_collection",
}

func gpwEditionKey(edition string) string {
	lower := strings.ToLower(strings.TrimSpace(edition))
	if lower == "" {
		return ""
	}
	if k, ok := gpwEditionMap[lower]; ok {
		return k
	}
	return "" // 拿不准不投（原始版本语义）
}

// gpwCheckboxFill 特性 checkbox 族（8 个——MI/标题/标签判定）。
func gpwCheckboxFill(meta *model.TorrentMetadata, tp titleparser.TechProfile, extra map[string]string) {
	// 视频特性（tp.HDR/tp.BitDepth）
	if tp.BitDepth == "10bit" || tp.BitDepth == "10" {
		extra["gpw_10_bit"] = "1"
	}
	hdrLower := strings.ToLower(tp.HDR)
	if strings.Contains(hdrLower, "hdr10+") || strings.Contains(hdrLower, "hdr10plus") {
		extra["gpw_hdr10plus"] = "1"
	} else if strings.Contains(hdrLower, "hdr10") {
		extra["gpw_hdr10"] = "1"
	}
	if strings.Contains(hdrLower, "dolby vision") || strings.Contains(hdrLower, "dovi") || strings.Contains(hdrLower, "dv") {
		extra["gpw_dolby_vision"] = "1"
	}

	// 音频特性（MI AudioTechnology/tp）
	miLower := strings.ToLower(meta.MediaInfo)
	if strings.Contains(miLower, "dolby digital plus with dolby atmos") ||
		strings.Contains(miLower, "eac3 Journal with dolby atmos") ||
		strings.Contains(miLower, "ddp atmos") {
		extra["gpw_dolby_atmos"] = "1"
	}
	if strings.Contains(miLower, "dts:x") || strings.Contains(miLower, "dts x") {
		extra["gpw_dts_x"] = "1"
	}

	// 国配/特效字幕（标签推断——meta.Tags JSON 字符串串判）
	if strings.Contains(meta.Tags, "chinese_audio") ||
		strings.Contains(meta.Tags, "国语") || strings.Contains(meta.Tags, "国配") {
		extra["gpw_chinese_dubbed"] = "1"
	}
	if strings.Contains(meta.Tags, "special_effects_subs") || strings.Contains(meta.Tags, "特效字幕") {
		extra["gpw_special_effects_subtitles"] = "1"
	}
}

// gpwFirst 首个非空串（包内私有——adapter 层 firstNonEmpty 同语义）。
func gpwFirst(ss ...string) string {
	for _, x := range ss {
		if x != "" {
			return x
		}
	}
	return ""
}

// gpwParsePTGen PTGen JSON 建组字段轻量解析（§59.462 附七——列空兜底源）。
type gpwPTGenFields struct {
	ChineseTitle string
	ForeignTitle string
	Introduction string
	IMDbURL      string
	IMDbID       string
	// Year §59.462 附七: PTGen 实测 "year":"1980" 字符串形态（非数字）
	// ——json.Number 双兼容（字符串/数字同解；int 形态遇字符串值
	// Unmarshal 整体失败致全零值——诊断5 实证）。
	Year  json.Number
	Genre []string
}

func gpwParsePTGen(src string) gpwPTGenFields {
	var f gpwPTGenFields
	if src == "" {
		return f
	}
	var d struct {
		ChineseTitle string   `json:"chinese_title"`
		ForeignTitle string   `json:"foreign_title"`
		Introduction string   `json:"introduction"`
		IMDbURL      string   `json:"imdb_url"`
		IMDbID       string   `json:"imdb_id"`
		Year         json.Number `json:"year"`
		Genre        []string `json:"genre"`
	}
	if err := json.Unmarshal([]byte(src), &d); err != nil {
		return f
	}
	return gpwPTGenFields(d)
}

// gpwImageHostWhitelist §59.462 站规 2.2.1 图床白名单（rules.php?p=upload
// 八域——gpw.md §一 权威）。匹配=host 等值或子域（img104.pixhost.to ∈
// pixhost.to）。
var gpwImageHostWhitelist = []string{
	"kshare.club", "pixhost.to", "ptpimg.me",
	"img.pterclub.net", "img.pterclub.com",
	"yes.ilikeshots.club", "imgbox.com", "s3.pterclub.com",
}

// gpwHostAllowed host 白名单判定（等值或子域后缀）。
func gpwHostAllowed(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// gpwScreenshotGate §59.462 附九: 站规 2.2.1 截图三要件门（质量门=拦截
// 语义——摸底定案"预检 URL 后缀判+重截重审"还原：门是拦截+文案给行动
// 路径，非 advisory 放行）：
//  1. 数量 ≥3 张（原始分辨率要件由截图引擎天然满足——从视频截取）
//  2. 格式 PNG（URL 后缀判——剥 query/fragment 后）
//  3. 图床 ∈ 白名单八域（子域匹配）
//
// DryRun 同拦（§59.409 附二先例——预览可见拦截）。
func gpwScreenshotGate(shots []string) (bool, string) {
	if len(shots) < 3 {
		return true, fmt.Sprintf("海豹站规 2.2.1：截图至少 3 张（当前 %d 张）——请先在 Tab3 生成截图", len(shots))
	}
	for _, raw := range shots {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Host == "" {
			return true, "海豹站规 2.2.1：截图 URL 无法解析（" + raw + "）"
		}
		host := strings.ToLower(u.Hostname())
		ok := false
		for _, d := range gpwImageHostWhitelist {
			if gpwHostAllowed(host, d) {
				ok = true
				break
			}
		}
		if !ok {
			return true, "海豹站规 2.2.1 图床白名单：" + host + " 不在八域内（kshare/pixhost/ptpimg/pterclub/ilikeshots/imgbox）——请更换图床配置后重截"
		}
		// 后缀判（剥 query/fragment）
		path := u.Path
		if i := strings.IndexAny(path, "?#"); i >= 0 {
			path = path[:i]
		}
		if !strings.EqualFold(strings.ToLower(path[len(path)-4:]), ".png") {
			return true, "海豹站规 2.2.1 要求 PNG 截图（" + path + " 为非 png）——设置→截图格式切 png→Tab3 重截→重发（重截后缓存自动写穿覆盖）"
		}
	}
	return false, ""
}

// gpwBuildArtists PTGen 剧组→四件套（导演 1+演员 6——SplitPersonName
// 拆分）。数据源：ptgen_source_json 的 director/cast 数组（§59.236 X
// 重构四键 JSON 派生——运行时解析）。
func gpwBuildArtists(ptgenJSON string) []model.PublishArtist {
	if ptgenJSON == "" {
		return nil
	}
	var src struct {
		Director []string `json:"director"`
		Cast     []struct {
			Name string `json:"name"`
		} `json:"cast"`
	}
	if err := json.Unmarshal([]byte(ptgenJSON), &src); err != nil {
		return nil
	}
	var out []model.PublishArtist
	for _, d := range src.Director {
		pn := util.SplitPersonName(d)
		if pn.Name != "" || pn.SubName != "" {
			out = append(out, model.PublishArtist{
				Name: pn.Name, SubName: pn.SubName, ID: "", Role: "1",
			})
		}
	}
	for i, a := range src.Cast {
		if i >= 5 {
			break
		}
		pn := util.SplitPersonName(a.Name)
		if pn.Name != "" || pn.SubName != "" {
			out = append(out, model.PublishArtist{
				Name: pn.Name, SubName: pn.SubName, ID: "", Role: "6",
			})
		}
	}
	return out
}

// isOriginalDisc 原盘族判定——MediumCanonicalOf 权威媒介判定的"原盘"
// 后缀（§59.462 附七: 首版仅 FromBDInfo 铁证过窄——BDInfo 数据缺失的
// 原盘（如借壳素材）恒 miss；标题启发式族（ST 连字符原盘写法）经
// MediumCanonicalOf 统一单点覆盖，压制误判防护由其内部 hasReencodeMark
// 保证）。
func isOriginalDisc(tp titleparser.TechProfile) bool {
	return strings.Contains(titleparser.MediumCanonicalOf(tp), "原盘")
}

// gpwLookupSize hash→snapshot size（碟规格判定数据源——§59.462）。
func (e *PublishExecutor) gpwLookupSize(ctx context.Context, hash string) int64 {
	var snap struct{ Size int64 }
	e.db.WithContext(ctx).Raw("SELECT size FROM torrent_snapshots WHERE hash = ? LIMIT 1", hash).Scan(&snap)
	return snap.Size
}
