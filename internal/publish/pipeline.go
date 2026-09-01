package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/compliance"
	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/event"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/imagehost"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/notification"
	"github.com/ranfish/pt-forward/internal/ptgen"
	"github.com/ranfish/pt-forward/internal/pusher"
	"github.com/ranfish/pt-forward/internal/screenshot"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var reTMDBID = regexp.MustCompile(`(?:themoviedb\.org|tmdb\.org)/(?:movie|tv)/(\d+)`)

type Pipeline struct {
	db                *gorm.DB
	logger            *zap.Logger
	siteProvider      model.SiteInfoProvider
	clientProvider    model.DownloaderProvider
	ptgen             *ptgen.Provider
	completionWatcher model.CompletionWatcher
	notifyService     *notification.Service
	screenshotConfig  *screenshot.Config
	backpressureCtrl  *BackpressureController
	artifactGenerator *PublishArtifactGenerator
	limitGuard        *PublishLimitGuard
	declarationFilter *DeclarationFilter
	complianceChecker *compliance.Checker
	metadataFetcher   *metadata.Fetcher
	imageHostStrategy string
	imageHostMgr      *imagehost.Manager
	pusher            *pusher.Pusher // §56.30: 发布后自动加种
	memberMu          sync.Map
	wsBroadcaster     event.WSBroadcaster
	bdinfoScanner     *BDInfoScanner
	// §59.146: TagApplier 灰度站点查询（nil=关闭；返回 settings 逗号分隔串）
	tagApplierSites func() string
}



func NewPipeline(db *gorm.DB, logger *zap.Logger) *Pipeline {
	logger = logger.With(zap.String("component", "publish"))
	return &Pipeline{
		db:        db,
		logger:    logger,
		ptgen:     ptgen.NewProvider(db, logger),
		limitGuard: NewPublishLimitGuard(db, logger),
	}
}

func (p *Pipeline) SetSiteProvider(sp model.SiteInfoProvider) {
	p.siteProvider = sp
}


func (p *Pipeline) SetClientProvider(cp model.DownloaderProvider) {
	p.clientProvider = cp
}

func (p *Pipeline) SetPTGenEndpoints(endpoints string) {
	if p.ptgen != nil {
		p.ptgen.SetEndpoints(endpoints)
	}
}

func (p *Pipeline) SetPTGenAPIKey(key string) {
	if p.ptgen != nil {
		p.ptgen.SetAPIKey(key)
	}
}

func (p *Pipeline) SetDeclarationFilter(df *DeclarationFilter) {
	p.declarationFilter = df
}

func (p *Pipeline) SetComplianceChecker(c *compliance.Checker) {
	p.complianceChecker = c
}

func (p *Pipeline) SetMetadataFetcher(f *metadata.Fetcher) {
	p.metadataFetcher = f
}

func (p *Pipeline) SetImageHostStrategy(strategy string) {
	if strategy != "" {
		p.imageHostStrategy = strategy
	}
}

func (p *Pipeline) SetWSBroadcaster(b event.WSBroadcaster) {
	p.wsBroadcaster = b
}

func (p *Pipeline) SetBDInfoScanner(s *BDInfoScanner) {
	p.bdinfoScanner = s
}

func (p *Pipeline) SetImageHostManager(mgr *imagehost.Manager) {
	p.imageHostMgr = mgr
	if p.artifactGenerator != nil {
		p.artifactGenerator.SetImageHostManager(mgr)
	}
}

// SetPusher §56.30: 注入 pusher 用于发布后自动加种。
func (p *Pipeline) SetPusher(pusher *pusher.Pusher) {
	p.pusher = pusher
}

// isDetailFirst §56.18: 从 publish_settings 读取海报优先级 toggle。
func (p *Pipeline) isDetailFirst() bool {
	if p.db == nil {
		return false
	}
	var setting model.PublishSetting
	if err := p.db.Where("key = ?", "metadata_priority").First(&setting).Error; err == nil {
		return setting.Value == "detail_first"
	}
	return false
}

// loadTitleRules §56.19: 从 DB 加载目标站的标题校验规则。
func (p *Pipeline) loadTitleRules(siteCode string) []model.TitleRule {
	if p.db == nil {
		return nil
	}
	var rules []model.TitleRule
	// 查全局规则（site_code=''）+ 目标站规则
	p.db.Where("site_code = ? OR site_code = ?", siteCode, "").Find(&rules)
	return rules
}

// getExistingStrategy §56.23: 从 DB 读取目标站的已存在种子策略。
func (p *Pipeline) getExistingStrategy(siteName string) model.ExistingStrategy {
	if p.db == nil {
		return model.ExistingSkip
	}
	var site model.Site
	p.db.Where("name = ? OR domain = ?", siteName, siteName).First(&site)
	return model.ParseExistingStrategy(site.ExistingStrategy)
}

func (p *Pipeline) SetCompletionWatcher(w model.CompletionWatcher) {
	p.completionWatcher = w
}

func (p *Pipeline) SetNotifyService(ns *notification.Service) {
	p.notifyService = ns
}

func (p *Pipeline) SetScreenshotConfig(cfg screenshot.Config) {
	p.screenshotConfig = &cfg
	p.artifactGenerator = NewPublishArtifactGenerator(&cfg, p.logger)
	// §56.17 决策 2: 如果 imageHostMgr 已设置，传递给新生成的 artifactGenerator
	if p.imageHostMgr != nil {
		p.artifactGenerator.SetImageHostManager(p.imageHostMgr)
	}
}

func (p *Pipeline) SetBackpressureController(ctrl *BackpressureController) {
	p.backpressureCtrl = ctrl
}

func (p *Pipeline) CreateTask(ctx context.Context, task *model.PublishTask) error {
	task.Status = model.PublishTaskPending
	return p.db.WithContext(ctx).Create(task).Error
}

func (p *Pipeline) GetTask(ctx context.Context, id uint) (*model.PublishTask, error) {
	var task model.PublishTask
	err := p.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (p *Pipeline) ListTasks(ctx context.Context, offset, limit int) ([]model.PublishTask, int64, error) {
	var tasks []model.PublishTask
	var total int64

	if err := p.db.WithContext(ctx).Model(&model.PublishTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := p.db.WithContext(ctx).Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&tasks).Error
	return tasks, total, err
}

func (p *Pipeline) Update(ctx context.Context, task *model.PublishTask) error {
	return p.db.WithContext(ctx).Save(task).Error
}

func (p *Pipeline) UpdateTaskStatus(ctx context.Context, id uint, status model.PublishTaskStatus) error {
	return p.db.WithContext(ctx).Model(&model.PublishTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

func (p *Pipeline) DeleteTask(ctx context.Context, id uint) error {
	return p.db.WithContext(ctx).Delete(&model.PublishTask{}, id).Error
}

func (p *Pipeline) CreateCandidate(ctx context.Context, candidate *model.PublishCandidate) error {
	return p.db.WithContext(ctx).Create(candidate).Error
}

func (p *Pipeline) GetCandidate(ctx context.Context, id uint) (*model.PublishCandidate, error) {
	var candidate model.PublishCandidate
	err := p.db.WithContext(ctx).First(&candidate, id).Error
	if err != nil {
		return nil, err
	}
	return &candidate, nil
}

func (p *Pipeline) DeleteCandidate(ctx context.Context, id uint) error {
	return p.db.WithContext(ctx).Delete(&model.PublishCandidate{}, id).Error
}

func (p *Pipeline) AnalyzePTGen(ctx context.Context, name string) (*model.PTGenResult, error) {
	if p.ptgen == nil {
		return nil, nil
	}
	return p.ptgen.Query(ctx, name)
}

// ptgenToMap 把 PTGenResult 字段填入 map（AnalyzeTorrent wrapper 向后兼容用）。
// 字段映射保持与 v0.0.278 之前的 AnalyzeTorrent 一致。
func ptgenToMap(r *model.PTGenResult, result map[string]interface{}) {
	if r == nil {
		return
	}
	if r.RawBBCode != "" {
		result["description"] = r.RawBBCode
	}
	if r.PosterURL != "" {
		result["poster_url"] = r.PosterURL
	}
	if r.DoubanURL != "" {
		result["douban_link"] = r.DoubanURL
	}
	if r.IMDBURL != "" {
		result["imdb_link"] = r.IMDBURL
	}
	if r.TMDbURL != "" {
		result["tmdb_link"] = r.TMDbURL
	}
	if r.ChineseTitle != "" {
		result["subtitle"] = r.ChineseTitle
	}
	if len(r.Genre) > 0 {
		result["ptgen_genre"] = strings.Join(r.Genre, ",")
	}
	if r.Episodes != "" {
		result["ptgen_episodes"] = r.Episodes
	}
}

// AnalyzeLocalArtifacts 只跑本地产物生成（截图 + MediaInfo）。
// §56.33 决策 A1：cachedMeta 命中时跳过此方法（用缓存本地产物），节省最耗时部分。
// §56.35 修复：用 name 模糊匹配精确定位种子文件/目录，避免在根目录找到其他种子的文件。
func (p *Pipeline) AnalyzeLocalArtifacts(ctx context.Context, name, savePath string) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"media_info":  "",
		"screenshots": []string{},
	}
	if p.artifactGenerator == nil || savePath == "" {
		return result, nil
	}

	// 精确定位种子内容路径
	// 先精确匹配 savePath/name，失败时用英文关键词模糊匹配
	// 避免在根目录中 findLargestVideo 找到其他种子的文件
	torrentDir := savePath
	if entryPath, isDir := findTorrentEntry(savePath, name); entryPath != "" {
		// 精确匹配到文件或目录
		// 文件路径：GenerateWithStrategy 会识别为文件直接分析
		// 目录路径：findLargestVideo 在种子目录内搜索
		torrentDir = entryPath
		_ = isDir
	}

	// 分析阶段用 source_direct 策略：只做 MediaInfo，跳过 mpv 截图（截图在发布阶段 buildPublishRequest 做）
	if artifact, err := p.artifactGenerator.GenerateWithStrategy(ctx, torrentDir, "", nil, "source_direct"); err == nil && artifact != nil {
		if artifact.MediaInfoText != "" {
			result["media_info"] = artifact.MediaInfoText
		}
		if len(artifact.ScreenshotURLs) > 0 {
			result["screenshots"] = artifact.ScreenshotURLs
		}
	} else if err != nil {
		p.logger.Warn("analyze: artifact generation failed", zap.Error(err))
	}
	return result, nil
}

// findTorrentEntry 在 dir 中查找 name 对应的种子内容路径。
// 先精确匹配 filepath.Join(dir, name)，失败时用英文关键词模糊匹配。
// 返回 (路径, 是否目录)。路径为空表示未找到。
func findTorrentEntry(dir, name string) (string, bool) {
	// 1. 精确匹配
	joined := filepath.Join(dir, name)
	if info, err := os.Stat(joined); err == nil {
		return joined, info.IsDir()
	}

	// 2. 模糊匹配：用 name 的英文关键词（normalize 去掉分隔符差异）
	keyword := extractMatchKeyword(name)
	if len(keyword) < 5 {
		return "", false
	}

	keywordNorm := normalizeKeyword(keyword)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		entryNorm := normalizeKeyword(entry.Name())
		if strings.Contains(entryNorm, keywordNorm) {
			fullpath := filepath.Join(dir, entry.Name())
			if info, err := os.Stat(fullpath); err == nil {
				return fullpath, info.IsDir()
			}
		}
	}

	return "", false
}

// extractMatchKeyword 从种子名提取英文关键词用于模糊匹配。
// 去掉中文前缀和扩展名，取第一个英文+数字+点号连续段（前 15 字符）。
func extractMatchKeyword(name string) string {
	if ext := filepath.Ext(name); ext != "" {
		name = name[:len(name)-len(ext)]
	}
	if strings.HasPrefix(name, "[") {
		if idx := strings.Index(name, "]"); idx > 0 {
			name = name[idx+1:]
		}
	}
	name = strings.TrimLeft(name, ".- _\u00a0")
	var b strings.Builder
	started := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			started = true
		} else if r == '.' && started {
			b.WriteRune(r)
		} else if started {
			break
		}
	}
	result := strings.TrimSuffix(b.String(), ".")
	if len(result) > 15 {
		result = result[:15]
	}
	return result
}

// normalizeKeyword 去掉所有非字母数字字符（小写），用于模糊匹配时忽略分隔符差异。
// "A.Long.Shot.2023" → "alongshot2023"
// "A Long Shot 2023" → "alongshot2023"
func normalizeKeyword(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// AnalyzeTorrent PTGen + 本地产物一体化分析（向后兼容 wrapper）。
// 新代码应分别调用 AnalyzePTGen / AnalyzeLocalArtifacts 以获得细粒度控制（§56.33）。
func (p *Pipeline) AnalyzeTorrent(ctx context.Context, name, savePath string) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"description": "",
		"poster_url":  "",
		"media_info":  "",
		"screenshots": []string{},
		"douban_link": "",
		"imdb_link":   "",
		"tmdb_link":   "",
		"subtitle":    "",
	}
	if ptgenResult, err := p.AnalyzePTGen(ctx, name); err == nil && ptgenResult != nil {
		ptgenToMap(ptgenResult, result)
	}
	if localResult, err := p.AnalyzeLocalArtifacts(ctx, name, savePath); err == nil {
		for k, v := range localResult {
			result[k] = v
		}
	}
	return result, nil
}

type piecesHashSearcher interface {
	SearchByPiecesHash(ctx context.Context, config *model.SiteConfig, piecesHashes []string) (map[string]int, error)
}

func (p *Pipeline) dedupByPiecesHash(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, torrentData []byte) (bool, string) {
	if len(torrentData) == 0 {
		return false, ""
	}

	meta, err := fingerprint.ComputeFromTorrent(torrentData)
	if err != nil || meta == nil || meta.PiecesHash == "" {
		return false, ""
	}

	if !adapter.SupportsSearchByPiecesHash() {
		return false, ""
	}

	searcher, ok := adapter.(piecesHashSearcher)
	if !ok {
		return false, ""
	}

	matches, err := searcher.SearchByPiecesHash(ctx, config, []string{meta.PiecesHash})
	if err != nil || len(matches) == 0 {
		return false, ""
	}

	if torrentID, found := matches[meta.PiecesHash]; found && torrentID > 0 {
		return true, meta.PiecesHash
	}

	return false, ""
}

type descResult struct {
	Text       string
	Subtitle   string // §56.20: 副标题（PTGen 渲染）
	IMDbLink   string
	DoubanLink string
	TMDBID     string
}

func (p *Pipeline) renderDescription(ctx context.Context, sourceSite, targetSite, title string, sourceDetail *model.TorrentDetail) descResult {
	descriptionText := ""
	if sourceDetail != nil {
		descriptionText = sourceDetail.Description
	}

	descData := &model.DescriptionData{
		SourceSite: sourceSite,
		Title:      title, // §59.20: 渲染器提取制作组名生成致谢
	}
	if sourceDetail != nil {
		descData.MediaInfoText = sourceDetail.MediaInfo
		descData.Screenshots = sourceDetail.Screenshots
		if sourceDetail.BDInfo != "" {
			FillBDInfo(descData, sourceDetail.BDInfo)
		}
	}

	var result descResult
	ptgenResult, ptgenErr := p.queryPTGen(ctx, title)
	// PTGen 字段处理（独立于海报选择）
	if ptgenErr == nil && ptgenResult != nil {
		if ptgenResult.RawBBCode != "" {
			descData.PTGenBody = ptgenResult.RawBBCode
		}
		descData.PTGen = ptgenResult // §56.16: 结构化 PTGen
		result.IMDbLink = ptgenResult.IMDBURL
		result.DoubanLink = ptgenResult.DoubanURL
		if ptgenResult.TMDbURL != "" {
			result.TMDBID = extractTMDBID(ptgenResult.TMDbURL)
		}
	}

	// §56.18: 海报选择（toggle 支持 ptgen_first/detail_first）
	ptgenPoster := ""
	if ptgenResult != nil {
		ptgenPoster = ptgenResult.PosterURL
	}
	detailPoster := ""
	if sourceDetail != nil {
		detailPoster = sourceDetail.PosterURL
	}
	if p.isDetailFirst() && detailPoster != "" {
		descData.PosterURL = p.rehostPoster(ctx, detailPoster)
	} else if ptgenPoster != "" {
		descData.PosterURL = ptgenPoster
	} else if detailPoster != "" {
		descData.PosterURL = p.rehostPoster(ctx, detailPoster)
	}

	// §56.20: 副标题渲染（PTGen 外文名 + 年份）
	if ptgenResult != nil && (ptgenResult.ForeignTitle != "" || ptgenResult.ChineseTitle != "") {
		result.Subtitle = description.RenderSubtitle("", description.SubtitleData{
			PTGenForeignTitle: ptgenResult.ForeignTitle,
			PTGenChineseTitle: ptgenResult.ChineseTitle,
			PTGenYear:         ptgenResult.Year,
		})
	}

	if descriptionText == "" && descData.PTGenBody != "" {
		descriptionText = descData.PTGenBody
	}

	var descConfig model.SiteDescConfig
	siteInfo, siteInfoErr := p.siteProvider.GetSiteInfo(ctx, targetSite)
	if siteInfoErr == nil && siteInfo != nil {
		siteConfig, cfgErr := p.siteProvider.GetSiteConfig(ctx, targetSite)
		if cfgErr == nil && siteConfig != nil {
			descConfig = siteConfig.Publish.Description
		}
	}

	// §56.28: 音乐站用独立描述模板（Gazelle 框架）
	if fw := p.getTargetFramework(ctx, targetSite); fw == "gazelle" {
		albumData := &description.MusicAlbumData{
			Title:       title,
			Year:        "",
			PosterURL:   descData.PosterURL,
			Description: descriptionText,
		}
		if ptgenResult != nil {
			albumData.Year = ptgenResult.Year
		}
		if musicDesc := description.FormatMusicDescription(albumData); musicDesc != "" {
			descriptionText = musicDesc
		}
	} else if descConfig.Format != "" || descConfig.TemplateOverride != "" {
		renderer := description.NewRenderer(descConfig.Format)
		if rendered, err := renderer.Render(descData, descConfig); err == nil && rendered != "" {
			descriptionText = rendered
		}
	}

	// §59.20: 感谢引言由渲染器 Render() 始终添加，不再手动 prepend（避免重复）

	result.Text = descriptionText
	return result
}

func populateFormFields(fields map[string]string, detail *model.TorrentDetail) {
	if detail == nil {
		return
	}
	if detail.Category != "" {
		fields["category"] = detail.Category
	}
	if detail.Source != "" {
		fields["source"] = detail.Source
	}
	if detail.Resolution != "" {
		fields["resolution"] = detail.Resolution
	}
	if detail.Codec != "" {
		fields["codec"] = detail.Codec
	}
	if detail.AudioCodec != "" {
		fields["audioCodec"] = detail.AudioCodec
	}
	if detail.Processing != "" {
		fields["processing"] = detail.Processing
	}
	if detail.ReleaseGroup != "" {
		fields["team"] = detail.ReleaseGroup
	}
	if detail.Region != "" {
		fields["region"] = detail.Region
	}
	if detail.IMDbID != "" {
		fields["imdb"] = detail.IMDbID
	}
}

// applyUserOverrides 将手动转发时用户编辑的字段覆盖到 PublishRequest
// UserOverrides 是 JSON 字符串，包含 subtitle/statement/poster/douban_link/imdb_link/tmdb_link/tags/media_info/screenshots/description/bdinfo
func applyUserOverrides(pubReq *model.PublishRequest, overridesJSON string) {
	if overridesJSON == "" {
		return
	}
	var overrides map[string]interface{}
	if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
		return
	}

	// 仅在用户提供了非空值时覆盖（不覆盖空值）
	if v, ok := overrides["subtitle"].(string); ok && v != "" {
		pubReq.Subtitle = v
	}
	if v, ok := overrides["description"].(string); ok && v != "" {
		pubReq.Description = v
	}
	if v, ok := overrides["media_info"].(string); ok && v != "" {
		pubReq.MediaInfo = v
	}
	if v, ok := overrides["bdinfo"].(string); ok && v != "" {
		pubReq.BDInfo = v
	}
	if v, ok := overrides["douban_link"].(string); ok && v != "" {
		pubReq.DoubanLink = v
	}
	if v, ok := overrides["imdb_link"].(string); ok && v != "" {
		pubReq.IMDbLink = v
	}
	if v, ok := overrides["tmdb_link"].(string); ok && v != "" {
		if pubReq.ExtraFields == nil {
			pubReq.ExtraFields = make(map[string]string)
		}
		pubReq.ExtraFields["tmdb_id"] = extractTMDBID(v)
	}
	// screenshots 是 []interface{}
	if screenshots, ok := overrides["screenshots"].([]interface{}); ok && len(screenshots) > 0 {
		var urls []string
		for _, s := range screenshots {
			if str, ok := s.(string); ok && str != "" {
				urls = append(urls, str)
			}
		}
		if len(urls) > 0 {
			pubReq.Screenshots = urls
		}
	}
	// tags 是 []interface{}
	if tags, ok := overrides["tags"].([]interface{}); ok && len(tags) > 0 {
		if pubReq.TagFields == nil {
			pubReq.TagFields = make(map[string]string)
		}
		for _, tag := range tags {
			if str, ok := tag.(string); ok && str != "" {
				pubReq.TagFields[str] = "1"
			}
		}
	}
	// §56.29: 匿名发布字段
	if v, ok := overrides["anonymous"].(bool); ok {
		pubReq.Anonymous = v
	}
}

// overridesString 从 UserOverrides JSON 中提取字符串值
func overridesString(overridesJSON, key string) (string, bool) {
	if overridesJSON == "" {
		return "", false
	}
	var overrides map[string]interface{}
	if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
		return "", false
	}
	if v, ok := overrides[key].(string); ok {
		return v, true
	}
	return "", false
}

// overridesBool 从 UserOverrides JSON 中提取布尔值（§56.27）
func overridesBool(overridesJSON, key string) (bool, bool) {
	if overridesJSON == "" {
		return false, false
	}
	var overrides map[string]interface{}
	if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
		return false, false
	}
	if v, ok := overrides[key].(bool); ok {
		return v, true
	}
	return false, false
}

// applyTitleComponents 用用户编辑的标题组件覆盖表单字段
// 走标准化路径：原始值 → 标准键 → 规范显示名 → 表单字段
func applyTitleComponents(pubReq *model.PublishRequest, overridesJSON string) {
	if overridesJSON == "" {
		return
	}
	var overrides map[string]interface{}
	if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
		return
	}
	tcRaw, ok := overrides["title_components"]
	if !ok || tcRaw == nil {
		return
	}
	tc, ok := tcRaw.(map[string]interface{})
	if !ok {
		return
	}

	// 构建 TitleComponents 并标准化
	components := titleparser.TitleComponents{
		Resolution:   getStringFromMap(tc, "resolution"),
		VideoCodec:   getStringFromMap(tc, "video_codec"),
		AudioCodec:   getStringFromMap(tc, "audio_codec"),
		Medium:       getStringFromMap(tc, "medium"),
		ReleaseGroup: getStringFromMap(tc, "release_group"),
	}
	profile := titleparser.TechProfileFromTitle(components)
	stdParams, _ := titleparser.StandardizeTechProfile(profile)

	// 用标准键逆向映射为规范显示名，再填入表单
	// 如果逆向映射失败（不在标准映射表中），回退到原始值
	if components.Resolution != "" {
		display := titleparser.ReverseLookup(stdParams.Resolution)
		if display == "" {
			display = components.Resolution
		}
		pubReq.FormFields["resolution"] = display
	}
	if components.VideoCodec != "" {
		display := titleparser.ReverseLookup(stdParams.VideoCodec)
		if display == "" {
			display = components.VideoCodec
		}
		pubReq.FormFields["codec"] = display
	}
	if components.AudioCodec != "" {
		display := titleparser.ReverseLookup(stdParams.AudioCodec)
		if display == "" {
			display = components.AudioCodec
		}
		pubReq.FormFields["audioCodec"] = display
	}
	if components.Medium != "" {
		display := titleparser.ReverseLookup(stdParams.Medium)
		if display == "" {
			display = components.Medium
		}
		pubReq.FormFields["source"] = display
	}
	if components.ReleaseGroup != "" {
		pubReq.FormFields["team"] = components.ReleaseGroup
	}
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func extractTechProfile(userOverrides, title string) titleparser.TechProfile {
	if userOverrides != "" {
		var ov struct {
			TechProfile     *titleparser.TechProfile `json:"tech_profile"`
			TitleComponents map[string]string        `json:"title_components"`
		}
		if err := json.Unmarshal([]byte(userOverrides), &ov); err == nil {
			if ov.TechProfile != nil {
				return *ov.TechProfile
			}
			if len(ov.TitleComponents) > 0 {
				return titleparser.TechProfileFromTitle(titleparser.TitleComponents{
					MainTitle:      ov.TitleComponents["main_title"],
					SeasonEpisode:  ov.TitleComponents["season_episode"],
					Year:           ov.TitleComponents["year"],
					Resolution:     ov.TitleComponents["resolution"],
					Medium:         ov.TitleComponents["medium"],
					VideoCodec:     ov.TitleComponents["video_codec"],
					AudioCodec:     ov.TitleComponents["audio_codec"],
					HDRFormat:      ov.TitleComponents["hdr_format"],
					SourcePlatform: ov.TitleComponents["source_platform"],
					BitDepth:       ov.TitleComponents["bit_depth"],
					ReleaseVersion: ov.TitleComponents["release_version"],
					ReleaseGroup:   ov.TitleComponents["release_group"],
					ChinesePrefix:  ov.TitleComponents["chinese_prefix"],
				})
			}
		}
	}
	return titleparser.ParseTitleTech(title)
}

func (p *Pipeline) ListAllCandidates(ctx context.Context, page, pageSize int, status, search string) ([]model.PublishCandidate, int64, error) {
	var total int64
	q := p.db.WithContext(ctx).Model(&model.PublishCandidate{})
	if status != "" {
		q = q.Where("publish_status = ?", status)
	}
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("torrent_name LIKE ? OR source_site LIKE ? OR target_sites LIKE ?", like, like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		p.logger.Warn("query failed", zap.Error(err))
	}

	var candidates []model.PublishCandidate
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&candidates).Error; err != nil {
		p.logger.Warn("query failed", zap.Error(err))
	}

	return candidates, total, nil
}

func (p *Pipeline) ListPendingCandidates(ctx context.Context, limit int) ([]model.PublishCandidate, error) {
	var candidates []model.PublishCandidate
	q := p.db.WithContext(ctx).
		Where("publish_status IN ?", []string{"pending", "downloading"}).
		Order("created_at ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&candidates).Error
	return candidates, err
}

func (p *Pipeline) UpdateCandidateStatus(ctx context.Context, id uint, status model.PublishCandidateStatus, result string) error {
	now := time.Now()
	r := p.db.WithContext(ctx).Model(&model.PublishCandidate{}).
		Where("id = ? AND publish_status != ?", id, status).
		Updates(map[string]interface{}{
			"publish_status": status,
			"publish_result": result,
			"updated_at":     now,
		})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		p.logger.Debug("candidate status already updated, CAS skip",
			zap.Uint("id", id),
			zap.String("status", string(status)))
	}
	return nil
}

func (p *Pipeline) MarkDownloadCompleted(ctx context.Context, id uint, savePath, filePath string) error {
	now := time.Now()
	return p.db.WithContext(ctx).Model(&model.PublishCandidate{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"download_completed": true,
			"completed_at":       &now,
			"local_save_path":    savePath,
			"local_file_path":    filePath,
			"publish_status":     model.CandidateCompleted,
			"updated_at":         now,
		}).Error
}

func (p *Pipeline) CreateResult(ctx context.Context, result *model.PublishResultRecord) error {
	return p.db.WithContext(ctx).Create(result).Error
}

func (p *Pipeline) ListResults(ctx context.Context, candidateID uint, limit int) ([]model.PublishResultRecord, error) {
	var results []model.PublishResultRecord
	q := p.db.WithContext(ctx).Order("created_at DESC")
	if candidateID > 0 {
		q = q.Where("candidate_id = ?", candidateID)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&results).Error
	return results, err
}

func (p *Pipeline) ListResultsFiltered(ctx context.Context, page, pageSize int, status, targetSite, trigger, startDate, endDate string) ([]model.PublishResultRecord, int64, error) {
	var results []model.PublishResultRecord
	var total int64
	q := p.db.WithContext(ctx).Model(&model.PublishResultRecord{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if targetSite != "" {
		q = q.Where("target_site = ?", targetSite)
	}
	if trigger != "" {
		q = q.Where("trigger = ?", trigger)
	}
	if startDate != "" {
		q = q.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("created_at <= ?", endDate+" 23:59:59")
	}
	if err := q.Count(&total).Error; err != nil {
		p.logger.Warn("query failed", zap.Error(err))
	}

	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&results).Error; err != nil {
		p.logger.Warn("query failed", zap.Error(err))
	}

	return results, total, nil
}

func containsAnyKeyword(text string, keywords []string) (string, bool) {
	// §56.2x: MatchKeyword 防误伤（ASCII 词边界）——与 compliance.Checker 行为一致
	for _, kw := range keywords {
		if compliance.MatchKeyword(kw, text) {
			return kw, true
		}
	}
	return "", false
}

func (p *Pipeline) checkForbiddenContent(texts []string) (bool, string) {
	for _, text := range texts {
		if text == "" {
			continue
		}
		if kw, found := containsAnyKeyword(text, compliance.AdultKeywords); found {
			return false, fmt.Sprintf("内容包含成人/色情关键词: %s (§30.5 规则 1)", kw)
		}
		if kw, found := containsAnyKeyword(text, compliance.ForbiddenTransferKeywords); found {
			return false, fmt.Sprintf("标题/副标题包含禁止转载关键词: %s (§30.5 规则 2)", kw)
		}
		for _, grp := range compliance.ForbiddenGroups {
			if compliance.MatchKeyword(grp, text) {
				return false, fmt.Sprintf("禁止转载小组资源: %s (§30.5 规则 3)", grp)
			}
		}
	}
	return true, ""
}

func (p *Pipeline) CheckPublishEligibility(ctx context.Context, candidate *model.PublishCandidate, targetSite string) (bool, string) {
	// §59.20: 第 0 层——源站映射检查（制作组不在 release_group_mappings 中 → 不可发布）
	// 仅当映射表有数据时生效（测试环境/全新安装跳过）
	var totalMappings int64
	p.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).Count(&totalMappings)
	if totalMappings > 0 {
		groupName := util.ExtractGroupName(candidate.TorrentName)
		if groupName == "" {
			return false, "无法提取制作组名（无组名资源不可发布，§59.20）"
		}
		var mappingCount int64
		p.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).
			Where("LOWER(group_name) = LOWER(?)", groupName).
			Count(&mappingCount)
		if mappingCount == 0 {
			return false, fmt.Sprintf("制作组 %s 不在源站映射表中，不可发布（§59.20）", groupName)
		}
	}

	// 1. 合规检查（compliance.Checker 优先，含成人/禁转/小组/用户关键词/站点黑名单）
	if p.complianceChecker != nil {
		result := p.complianceChecker.CheckWithSite(ctx, candidate.TorrentName, candidate.SourceSite)
		if !result.Passed {
			return false, fmt.Sprintf("compliance_blocked:%s — %s", result.Category, result.Reason)
		}
	} else if eligible, reason := p.checkForbiddenContent([]string{candidate.TorrentName}); !eligible {
		return false, reason
	}

	if candidate.HasHR {
		return false, "源站种子存在 H&R (Hit and Run) 标记，跳过发布"
	}

	// §56.19: 标题校验（TitleValidator，如果目标站有标题规则）
	if rules := p.loadTitleRules(targetSite); len(rules) > 0 {
		validator := titleparser.NewTitleValidator()
		tpRules := make([]titleparser.TitleRule, len(rules))
		for i, r := range rules {
			tpRules[i] = titleparser.TitleRule{
				RuleType:     r.RuleType,
				Field:        r.Field,
				Pattern:      r.Pattern,
				Replacement:  r.Replacement,
				AutoFix:      r.AutoFix,
				ErrorMessage: r.ErrorMessage,
			}
		}
		vResult := validator.Validate(candidate.TorrentName, tpRules)
		if len(vResult.Errors) > 0 {
			msgs := make([]string, len(vResult.Errors))
			for i, e := range vResult.Errors {
				msgs[i] = e.Message
			}
			return false, "标题校验失败: " + strings.Join(msgs, "; ")
		}
		// 自动修复后的标题覆盖
		if vResult.Title != candidate.TorrentName {
			candidate.TorrentName = vResult.Title
		}
	}

	// 2. flags 检查（来自 torrent_metadata）
	if p.checkFlagsFromMetadata(ctx, candidate.InfoHash, candidate.SourceSite) {
		return false, fmt.Sprintf("源站 flags 标记禁转/独占（torrent_metadata），跳过发布")
	}

	// 3. declaration_filter 检查（flags 不可用时兜底）
	if p.declarationFilter != nil {
		texts := []string{candidate.TorrentName}
		patterns := p.declarationFilter.GetPatterns(ctx)
		if len(patterns) > 0 {
			fr := p.declarationFilter.Filter(strings.Join(texts, " "), patterns)
			if len(fr.RemovedDecls) > 0 {
				return false, fmt.Sprintf("声明过滤命中: %v", fr.RemovedDecls)
			}
		}
	}

	// 硬编码互斥站点对（不可修改）
	if IsHardcodedExclusion(candidate.SourceSite, targetSite) {
		return false, fmt.Sprintf("源站 %s → 目标站 %s 为互斥站点（硬编码规则）", candidate.SourceSite, targetSite)
	}

	// 用户自定义排除规则
	if targetSite != "" && candidate.SourceSite != "" {
		var exclusion model.PublishExclusion
		err := p.db.WithContext(ctx).
			Where("target_site = ? AND source_site = ?", targetSite, candidate.SourceSite).
			First(&exclusion).Error
		if err == nil {
			return false, fmt.Sprintf("源站 %s → 目标站 %s 存在发布排除规则", candidate.SourceSite, targetSite)
		}
	}

	// 加种限制 Guard
	if p.limitGuard != nil {
		if allowed, reason := p.limitGuard.Check(ctx, targetSite); !allowed {
			return false, fmt.Sprintf("目标站 %s 加种限制: %s", targetSite, reason)
		}
	}

	return true, ""
}

func (p *Pipeline) checkFlagsFromMetadata(ctx context.Context, infoHash, siteName string) bool {
	if infoHash == "" {
		return false
	}
	var meta model.TorrentMetadata
	if err := p.db.WithContext(ctx).
		Where("info_hash = ?", infoHash).
		Order("updated_at DESC").
		First(&meta).Error; err != nil {
		return false
	}
	if meta.Flags == "" {
		return false
	}
	var flags []string
	if err := json.Unmarshal([]byte(meta.Flags), &flags); err != nil {
		return false
	}
	// §59.162: 限时禁转两态——now<until 才拦（24h 窗到期自动放行）
	if containsStr(flags, "限时禁转") {
		if meta.NoTransferUntil == nil || meta.NoTransferUntil.After(time.Now()) {
			return true
		}
	}
	forbiddenFlags := map[string]bool{
		"禁转": true, "禁止转载": true, "谢绝转载": true,
		"严禁转载": true, "谢绝搬运": true, "独占": true,
		"adult": true,
	}
	for _, f := range flags {
		if forbiddenFlags[f] {
			return true
		}
	}
	return false
}

// hardcodedExclusionPairs 硬编码互斥站点对（双向），禁止用户修改
var HardcodedExclusionPairs = map[string]map[string]bool{
	"不可说": {"优堡": true},
	"优堡":   {"不可说": true},
	"家园":   {"铂金家": true},
	"铂金家": {"家园": true},
}

func PublishHardcodedExclusionPairs() map[string]map[string]bool {
	return HardcodedExclusionPairs
}

func IsHardcodedExclusion(sourceSite, targetSite string) bool {
	if targets, ok := HardcodedExclusionPairs[sourceSite]; ok {
		return targets[targetSite]
	}
	return false
}

func parseTargetSites(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func (p *Pipeline) OnTorrents(ctx context.Context, events []model.TorrentEvent) error {
	// §59.147: 旧自动发布流拆除（R3-2 手动起步定案——发布页显式提交是唯一入口；
	// fetchSourceInfo 现场拉取违反 §59.121 零拉取）。保留接口满足 eventbus handler 签名。
	_ = events
	return nil
}

func (p *Pipeline) queryPTGen(ctx context.Context, title string) (*model.PTGenResult, error) {
	if p.ptgen == nil {
		return nil, nil
	}
	if title == "" {
		return nil, nil
	}
	result, err := p.ptgen.Query(ctx, title)
	if err != nil {
		p.logger.Debug("ptgen query skipped",
			zap.String("title", title),
			zap.Error(err),
		)
		return nil, err
	}
	return result, nil
}

func extractTMDBID(tmdbURL string) string {
	m := reTMDBID.FindStringSubmatch(tmdbURL)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func (p *Pipeline) mapFieldValues(ctx context.Context, targetSite string, fields map[string]string) {
	var mappings []model.SiteFieldMapping
	if err := p.db.WithContext(ctx).
		Where("site_name = ?", targetSite).
		Find(&mappings).Error; err != nil || len(mappings) == 0 {
		return
	}

	fieldMap := make(map[string]map[string]string)
	for _, m := range mappings {
		if _, ok := fieldMap[m.FieldType]; !ok {
			fieldMap[m.FieldType] = make(map[string]string)
		}
		fieldMap[m.FieldType][m.SourceValue] = m.TargetValue
	}

	mapKey := func(fieldType, value string) string {
		if m, ok := fieldMap[fieldType]; ok {
			if mapped, ok := m[value]; ok {
				return mapped
			}
		}
		return value
	}

	if v, ok := fields["category"]; ok {
		fields["category"] = mapKey("cat", v)
	}
	if v, ok := fields["resolution"]; ok {
		mapped := mapKey("standard_sel", v)
		if mapped == v {
			mapped = mapKey("resolution", v)
		}
		fields["resolution"] = mapped
	}
	if v, ok := fields["codec"]; ok {
		mapped := mapKey("codec_sel", v)
		if mapped == v {
			mapped = mapKey("videoCodec", v)
		}
		fields["codec"] = mapped
	}
	if v, ok := fields["source"]; ok {
		mapped := mapKey("source_sel", v)
		if mapped == v {
			mapped = mapKey("source", v)
		}
		fields["source"] = mapped
	}
	if v, ok := fields["audioCodec"]; ok {
		mapped := mapKey("audiocodec_sel", v)
		if mapped == v {
			mapped = mapKey("audioCodec", v)
		}
		fields["audioCodec"] = mapped
	}
	if v, ok := fields["team"]; ok {
		mapped := mapKey("team_sel", v)
		if mapped == v {
			mapped = mapKey("team", v)
		}
		fields["team"] = mapped
	}
	if v, ok := fields["medium"]; ok {
		mapped := mapKey("medium_sel", v)
		if mapped == v {
			mapped = mapKey("medium", v)
		}
		fields["medium"] = mapped
	}
}

func (p *Pipeline) getTargetFramework(ctx context.Context, targetSite string) string {
	if p.siteProvider == nil {
		return ""
	}
	if siteInfo, err := p.siteProvider.GetSiteInfo(ctx, targetSite); err == nil && siteInfo != nil {
		return string(siteInfo.Framework)
	}
	return ""
}

func (p *Pipeline) rehostPoster(ctx context.Context, sourceURL string) string {
	if sourceURL == "" {
		return ""
	}
	if p.imageHostMgr == nil || p.imageHostMgr.DefaultHost() == nil {
		return sourceURL
	}
	result, err := p.imageHostMgr.Rehost(ctx, sourceURL)
	if err != nil || result == nil || result.URL == "" {
		p.logger.Debug("poster rehost failed, using source URL",
			zap.String("source_url", sourceURL),
			zap.Error(err))
		return sourceURL
	}
	p.logger.Debug("poster rehosted",
		zap.String("source_url", sourceURL),
		zap.String("rehosted_url", result.URL))
	return result.URL
}

// CaptureScreenshots §59.50: 本地 mpv 截图（Tab3 "重新获取截图（mpv）" 专用）。
//
// 与 AnalyzeLocalArtifacts 的区别：后者是 MI 场景（source_direct 跳过截图），
// 本方法走 local_upload 策略——mpv 截图 + 字幕检测 + HDR tone-mapping + 图床上传，
// 失败回源站截图（调用方传 sourceScreenshots 作 fallback）。
// 返回截图 URL 列表（本地截图全失败且无源站值时为空列表）。
func (p *Pipeline) CaptureScreenshots(ctx context.Context, name, savePath string, sourceScreenshots []string) []string {
	if p.artifactGenerator == nil || savePath == "" {
		return sourceScreenshots
	}
	torrentDir := savePath
	if entryPath, _ := findTorrentEntry(savePath, name); entryPath != "" {
		torrentDir = entryPath
	}
	artifact, err := p.artifactGenerator.GenerateWithStrategy(ctx, torrentDir, "", sourceScreenshots, "local_upload")
	if err != nil || artifact == nil {
		p.logger.Warn("capture screenshots failed", zap.Error(err))
		return sourceScreenshots
	}
	return artifact.ScreenshotURLs
}

// ApplyScreenshotStrategy §59.53: 采集链截图策略——按库内截图值跑 auto
// （白名单逐张/转存/差额补足/无图全量），isLocal=false 时只转存不截图（远程无图留空）。
// 返回最终截图列表（落库由调用方执行）。
func (p *Pipeline) ApplyScreenshotStrategy(ctx context.Context, name, savePath string, sourceScreenshots []string, isLocal bool) []string {
	if p.artifactGenerator == nil {
		return sourceScreenshots
	}
	torrentDir := savePath
	if entryPath, _ := findTorrentEntry(savePath, name); entryPath != "" {
		torrentDir = entryPath
	}
	if !isLocal {
		// §59.53 第6点: 远程只转存（白名单逐张判定），无图留空——不截图
		return p.artifactGenerator.ProcessScreenshotsRemote(sourceScreenshots)
	}
	artifact, err := p.artifactGenerator.GenerateWithStrategy(ctx, torrentDir, "", sourceScreenshots, "auto")
	if err != nil || artifact == nil {
		return sourceScreenshots
	}
	return artifact.ScreenshotURLs
}
