package publish

import (
	"context"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.514 刀2: .torrent 证据层——发布时 ExportTorrent 在手的 torrentData
// （executor :291 零新增 RPC）解析 name+files[]，与标题 tp 的季/年/来源/
// 品牌逐维仲裁。站方审核判据实证=标题 token vs 实际文件 token（优堡四
// 案例拒收：集数/季数/来源与实际文件不符）。
//
// 证据分层：文件树（事实）> .torrent name（发布组命名权威）> 源站标题
// > 回流标题。保守原则：文件 token miss 不覆盖（文件名无 S/E/年 token
// 时标题值保留）；技术参数（codec/声道/位深）不参与——MI 元数据段既有
// 纠错链权威（文件名 token 是发布者写法，Quacks 案文件名标 AC3 实为 DDP）。
//
// 与 NormalizeSeasonPack 的边界（讨论定案）：归一=区间形态+声明信号
// （tags complete/副标全N集/PTGen episodes → S01）；仲裁=事实构造
// （季集合跨季/裸单集错标修正/年份区间/来源/品牌）——两层互补不重叠。

var (
	// arbVideoExt 视频后缀（classify.videoExtensions 同款——发布链侧副本）
	arbVideoExt = map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".ts": true, ".m2ts": true, ".wmv": true, ".flv": true, ".mov": true}
	// arbFileSE 文件名季集 token（S01E03——Time 案文件树）
	arbFileSE = regexp.MustCompile(`(?i)S(\d{1,2})E(\d{1,3})`)
	// arbFileE 文件名裸集 token（E01——Quacks 案文件树）
	arbFileE = regexp.MustCompile(`(?i)(?:^|[\s.\-_])E(\d{1,3})(?:[\s.\-_]|$)`)
	// arbFileYear 文件名年份 token
	arbFileYear = regexp.MustCompile(`(?:19|20)\d{2}`)
	// arbSubAllEps 副标全N集（§59.424 P2 词形——归一通道 C）
	arbSubAllEps = regexp.MustCompile(`全\s*([0-9]{1,4}|[零一二三四五六七八九十百千]{1,4})\s*集|(\d{1,4})集全`)
)

// arbitrateWithTorrent 执行仲裁并就地修正 tp。返回仲裁记录（DryRun 透出+
// Info 日志可观测）。torrentData 空时 no-op（降级链：副标/tags 词形信号现状）。
func (e *PublishExecutor) arbitrateWithTorrent(ctx context.Context, meta *model.TorrentMetadata, tp *titleparser.TechProfile, torrentData []byte) []string {
	if meta == nil || tp == nil || len(torrentData) == 0 {
		return nil
	}
	tm, err := fingerprint.ComputeFromTorrent(torrentData)
	if err != nil || tm == nil {
		return nil // 解析失败=证据缺席——保守不干预（词形信号链兜底）
	}
	var notes []string

	// ① .torrent name 权威仲裁（发布组命名——五案全对的证据源）
	if tm.Name != "" {
		notes = append(notes, e.arbitrateNameTokens(meta, tp, tm.Name)...)
	}
	// ② 文件树证据（事实层——季集合构造/裸单集错标/年份区间）
	notes = append(notes, e.arbitrateFileTree(tp, tm.FileTree)...)
	// ③ raw_name 落库（§59.252 数学锚点——顺路补全·发布链写 meta 列
	// 有 english_title 先例（mergePTGenSource）非网络动作）
	if meta.TorrentRawName == "" && tm.Name != "" && meta.InfoHash != "" && e.db != nil {
		_ = e.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ?", meta.InfoHash).
			Update("torrent_raw_name", tm.Name).Error
		meta.TorrentRawName = tm.Name
	}
	return notes
}

// arbitrateNameTokens name 的 S/E/年/来源/品牌 token 与标题 tp 仲裁。
// name 是 .torrent name（发布组命名——下载器做种目录名的数学权威版）。
func (e *PublishExecutor) arbitrateNameTokens(meta *model.TorrentMetadata, tp *titleparser.TechProfile, name string) []string {
	var notes []string
	np := titleparser.BuildTechProfile(name, "", "", "", "", "")

	// S/E：name 形态更完整时修正（Time：S01-S02 vs 标题 S01；
	// Quacks/Book：S01 vs 标题裸 E01——下载器名合集声明权威）
	nameForm := titleparser.ParseSEForm(name)
	titleForm := titleparser.ClassifySEForm(tp.SeasonEpisode)
	if nameForm.SeasonEpisode != "" && nameForm.SeasonEpisode != tp.SeasonEpisode {
		nameIsPack := nameForm.Form == titleparser.FormSeasonPack || nameForm.Form == titleparser.FormCrossSeason
		titleIsPack := titleForm.Form == titleparser.FormSeasonPack || titleForm.Form == titleparser.FormCrossSeason
		// name=合集形态 ∧（标题非合集形态或标题形态更窄）→ name 赢
		if nameIsPack && (!titleIsPack || (titleForm.Form == titleparser.FormSeasonPack && nameForm.Form == titleparser.FormCrossSeason)) {
			old := tp.SeasonEpisode
			tp.SeasonEpisode = nameForm.SeasonEpisode
			notes = append(notes, "季集仲裁(name): "+old+" → "+nameForm.SeasonEpisode)
		}
	}

	// 年份：name 区间/单年 vs 标题（Time：2021-2023 vs 2021）
	if np.Year != "" && np.Year != tp.Year {
		nameHasRange := strings.Contains(np.Year, "-")
		titleHasRange := strings.Contains(tp.Year, "-")
		if nameHasRange && !titleHasRange {
			oldY := tp.Year
			tp.Year = np.Year
			notes = append(notes, "年份仲裁(name): "+oldY+" → "+np.Year)
		}
	}

	// §59.514 附二修正（用户定案）：WEB 族内细分（WEB-DL vs WEBRip）
	// name 不参与——From 案实证发布组命名不可信（name/文件名标 WEB-DL
	// 实为压制·MI Writing library 铁证；站方按文件名字面审核不核对压制
	// 参数·From 全系列 WEBRip 形态 10/11 站过审）。权威=MI 压制铁证
	// 单方向纠正（§59.226 附五单方向性——铁证只能证【是压制】，
	// MIEncoded=false 不反向纠正）。
	if strings.EqualFold(tp.Specification, "WEB-DL") && tp.MIEncoded {
		tp.Specification = "WEBRip"
		notes = append(notes, "规格仲裁(MI铁证): WEB-DL → WEBRip（压制痕迹 Writing library 在场）")
	}
	// 跨族媒介 Spec 冲突——name 证据（WEB 族内冲突已上行排除；
	// Mauvais sang 型媒介级错标由 ST 分支承载）
	if np.Specification != "" && tp.Specification != "" &&
		!strings.EqualFold(np.Specification, tp.Specification) &&
		!(isWebSpec(tp.Specification) && isWebSpec(np.Specification)) {
		old := tp.Specification
		tp.Specification = np.Specification
		notes = append(notes, "规格仲裁(name): "+old+" → "+np.Specification)
	}
	if np.SourceType != "" && tp.SourceType != "" &&
		!strings.EqualFold(np.SourceType, tp.SourceType) && np.Specification == "" {
		// ST 单独冲突仅当 Spec 无值时（Spec 承载 ST 语义的多数场景已上行处理）
		oldST, oldSpec := tp.SourceType, tp.Specification
		tp.SourceType = np.SourceType
		// §59.514 复验缺陷修：媒介级冲突（BluRay vs WEB-DL）ST 修正后
		// 旧媒介族 Spec 残留——重组双媒介 token（Mauvais sang 案）。
		// ST 已改判媒介→旧族 Spec 同源失真，一并清除交 ST 单独表达。
		if oldSpec != "" && !strings.EqualFold(oldSpec, np.SourceType) {
			tp.Specification = ""
		}
		notes = append(notes, "片源仲裁(name): "+oldST+" → "+np.SourceType+
			"（清残留规格 "+oldSpec+"）")
	}
	if np.SourcePlatform != "" && tp.SourcePlatform == "" {
		tp.SourcePlatform = np.SourcePlatform
		notes = append(notes, "分发方补全(name): "+np.SourcePlatform)
	}

	// 品牌（§59.514 CC 维）：标题无 ∧ name 有 → 补全（缺失补全语义——
	// Railroad CC 案；反方向标题有 name 无=保留标题）
	if np.EditionInfo != "" && tp.EditionInfo == "" {
		tp.EditionInfo = np.EditionInfo
		notes = append(notes, "品牌补全(name): "+np.EditionInfo)
	}
	return notes
}

// arbitrateFileTree 文件树证据（事实层）。季集合构造（Time 跨季发现）/
// 裸单集错标修正（Quacks）/年份区间（2021-2023）。文件 token miss 不覆盖。
func (e *PublishExecutor) arbitrateFileTree(tp *titleparser.TechProfile, fileTree map[string]int64) []string {
	if len(fileTree) == 0 {
		return nil
	}
	var notes []string
	seasons := map[string]bool{}
	epsBySeason := map[string][]int{} // 季→集号列表（""=无季前缀文件）
	bareEps := []int{}
	years := map[string]bool{}
	videoCount := 0
	for f := range fileTree {
		base := path.Base(f)
		if !arbVideoExt[strings.ToLower(path.Ext(base))] {
			continue
		}
		videoCount++
		if m := arbFileSE.FindStringSubmatch(base); m != nil {
			s := "S" + m[1]
			ep, _ := strconv.Atoi(m[2])
			seasons[s] = true
			epsBySeason[s] = append(epsBySeason[s], ep)
		} else if m := arbFileE.FindStringSubmatch(base); m != nil {
			ep, _ := strconv.Atoi(m[1])
			bareEps = append(bareEps, ep)
		}
		for _, y := range arbFileYear.FindAllString(base, -1) {
			years[y] = true
		}
	}

	// 季集合构造：文件树多季（Time：S01×3+S02×3）∧ 标题单季 → 跨季修正
	if len(seasons) >= 2 {
		ss := make([]string, 0, len(seasons))
		for s := range seasons {
			ss = append(ss, s)
		}
		sort.Slice(ss, func(i, j int) bool {
			a, _ := strconv.Atoi(ss[i][1:])
			b, _ := strconv.Atoi(ss[j][1:])
			return a < b
		})
		cross := ss[0] + "-" + ss[len(ss)-1]
		if tp.SeasonEpisode != cross {
			tp.SeasonEpisode = cross
			notes = append(notes, "跨季构造(文件树): "+strings.Join(ss, "+")+" → "+cross)
		}
	}

	// 裸单集错标修正（Quacks：标题裸 E01 分集形态 ∧ 文件树 N≥2 视频且
	// 裸集区间 E01 起 → 事实=单季合集 → v1.05 写法 S01）
	if videoCount >= 2 && len(bareEps) >= 2 && tp.SeasonEpisode != "" {
		sef := titleparser.ClassifySEForm(tp.SeasonEpisode)
		if sef.Form == titleparser.FormSingleEpisode {
			sort.Ints(bareEps)
			if bareEps[0] == 1 {
				tp.SeasonEpisode = "S01"
				notes = append(notes, "合集修正(文件树): 裸单集 E01 标记 × "+strconv.Itoa(len(bareEps))+" 文件完整区间 → S01")
			}
		}
	}

	// 年份区间构造：文件树多年（Time：2021+2023）∧ 标题单年 → 区间
	if len(years) >= 2 {
		ys := make([]string, 0, len(years))
		for y := range years {
			ys = append(ys, y)
		}
		sort.Strings(ys)
		range_ := ys[0] + "-" + ys[len(ys)-1]
		if tp.Year != "" && !strings.Contains(tp.Year, "-") {
			tp.Year = range_
			notes = append(notes, "年份区间(文件树): "+strings.Join(ys, "+")+" → "+range_)
		}
	}
	return notes
}

// isWebSpec WEB 族规格形态（WEB-DL/WEBRip——族内细分由 MI 铁证判定，
// name 证据不参与·§59.514 附二）。
func isWebSpec(s string) bool {
	return strings.EqualFold(s, "WEB-DL") || strings.EqualFold(s, "WEBRip")
}

// subtitleCompleteSignal 副标完结信号（§59.424 P2 词形——归一通道 C 扩展：
// 副标「全N集/N集全」与 tags complete 同权参与 NormalizeSeasonPack）。
func subtitleCompleteSignal(subtitle string) bool {
	return subtitle != "" && arbSubAllEps.MatchString(subtitle)
}
