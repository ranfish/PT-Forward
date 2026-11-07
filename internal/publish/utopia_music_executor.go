package publish

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/pusher"
)

// §59.485 刀2: 音乐种发布分支（乌托邦第一站）。
//
// 分流自 Execute（音乐种提前分流·影视链零触碰——见 executor.go 分流点
// 注释）。翻译层=utopia_music.go 纯函数；上传/结果段内联自主链
// （dedup 双路/§59.470 超时复查/§59.159 existing 分流/加种/落库）。
//
// ⚠️ 主链同步债：主链上传段行为变更时需同步本函数（稳定后收敛公共函数）。
func (e *PublishExecutor) executeMusicPublish(ctx context.Context, in ExecuteInput,
	meta *model.TorrentMetadata, rv *ResourceView, cfg *model.PublishFormConfig,
	torrentData []byte, mm *model.MusicMeta) *ExecuteResult {

	fail := func(status, msg string) *ExecuteResult {
		return &ExecuteResult{Status: status, Message: msg}
	}
	failRec := func(status, msg string) *ExecuteResult {
		if !in.DryRun {
			e.recordResultFull(ctx, in, meta, rv, status, msg, "", "", false)
		}
		return fail(status, msg)
	}

	// ① R7 音乐版（决策点⑤：最小预检集——R7 禁转拦+reviewed 门）
	if blocked, reason := utopiaMusicR7Blocked(meta, mm); blocked {
		return failRec("ineligible", reason)
	}

	// ② 翻译层组装（utopia_music.go 纯函数）
	groupName := utopiaMusicGroupName(meta, mm)
	name := utopiaMusicName(meta, mm, groupName)
	if strings.TrimSpace(name) == "" {
		return failRec("failed", "音乐标题组装失败（artist/album 全空）——检查获取数据")
	}
	subtitle := utopiaMusicSubtitle(mm)
	descr := utopiaMusicDescr(meta, mm)

	form := utopiaMusicFormValues(meta, mm)
	form["name"] = name
	if subtitle != "" {
		form["small_descr"] = subtitle
	}
	form["descr"] = descr

	// tags[4][] + logtexts[]（TagArrayFields 同名重复通道 §59.156）
	var tagArray []model.TagKV
	for _, v := range utopiaMusicTags(meta, mm, groupName) {
		tagArray = append(tagArray, model.TagKV{Key: "tags[4][]", Value: v})
	}
	for _, seg := range utopiaMusicLogTexts(mm) {
		tagArray = append(tagArray, model.TagKV{Key: "logtexts[]", Value: seg})
	}

	// ③ 必选域 fail-fast（audiocodec——映射不出不发出去·§59.289 同语义）
	if form["audiocodec_sel[4]"] == "" {
		return failRec("failed", fmt.Sprintf(
			"音频编码映射缺失：format=%q 不在乌托邦值集——检查获取数据 format 字段", mm.Format))
	}

	// ④ DryRun 检查点（组装后·上传前——完整表单形态验证）
	if in.DryRun {
		dryForm := map[string]string{}
		for k, v := range form {
			dryForm[k] = v
		}
		for _, kv := range tagArray {
			dryForm["["+kv.Key+"]"+fmt.Sprint(len(dryForm))] = kv.Value
		}
		return &ExecuteResult{
			Status: "dry_run_ok",
			Form:   dryForm,
			Tags:   utopiaMusicTags(meta, mm, groupName),
		}
	}

	// ⑤ 审核门（§59.273 同款收口——音乐种 reviewed 必须 true·§59.484
	// 强制审核过审的种同样满足）
	if !meta.Reviewed {
		return failRec("unreviewed", "种子未审核（reviewed=false）——请先在种子详情页完成审核")
	}

	// ⑥ pubReq 构造
	if cfg.Anonymous {
		in.Anonymous = true
	}
	pubReq := &model.PublishRequest{
		TorrentData:    torrentData,
		FormFields:     form,
		TagArrayFields: tagArray,
		Title:          name,
		Subtitle:       subtitle,
		Description:    descr,
		Anonymous:      in.Anonymous,
		SourceSite:     meta.SiteName,
		SourceInfoHash: in.InfoHash,
		TargetSite:     in.TargetSite,
		ClientUID:      rv.ClientUID,
	}

	// ⑦ 上传+结果段（内联自主链——dedup 双路/上传/超时复查/existing/
	// 加种/落库。⚠️ 主链同步债见文件头注释）
	adapter, aErr := e.pipe.siteProvider.GetAdapter(ctx, in.TargetSite)
	if aErr != nil {
		return failRec("failed", fmt.Sprintf("获取站点适配器失败: %v", aErr))
	}
	siteConfig, scErr := e.pipe.siteProvider.GetSiteConfig(ctx, in.TargetSite)
	if scErr != nil {
		return failRec("failed", fmt.Sprintf("获取站点配置失败: %v", scErr))
	}

	piecesHash := fingerprintPiecesHash(torrentData)
	if piecesHash != "" {
		var rec model.PublishResultRecord
		if err := e.db.WithContext(ctx).
			Where("target_site = ? AND pieces_hash = ? AND status IN ?", in.TargetSite, piecesHash,
				[]string{"pushed", "pushed_existing", "uploaded", "duplicate"}).
			Order("id DESC").First(&rec).Error; err == nil {
			msg := "本地记忆命中（站上已有同内容种）: " + piecesHash
			e.recordResult(ctx, in, meta, rv, "duplicate", msg, rec.TorrentID, detailURLOf(siteConfig, rec.TorrentID), piecesHash)
			return fail("duplicate", msg)
		}
	}
	if dup, dupMsg, mTID := e.pipe.dedupByPiecesHashFull(ctx, adapter, siteConfig, torrentData); dup {
		msg := "目标站已存在同内容种子: " + dupMsg
		tidShown := ""
		if mTID > 0 {
			tidShown = fmt.Sprintf("%d", mTID)
		}
		e.recordResult(ctx, in, meta, rv, "duplicate", msg, tidShown, detailURLOf(siteConfig, tidShown), piecesHash)
		return fail("duplicate", msg)
	}

	resp, upErr := adapter.UploadTorrent(ctx, siteConfig, pubReq)
	timeoutRecheckMsg := ""
	if upErr != nil {
		// §59.470 同款：超时≠失败——站方复查（pieces-hash）
		if isUploadTimeoutErr(upErr) {
			recheckCtx, recheckCancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
			dup, dupMsg, mTID := e.pipe.dedupByPiecesHashFull(recheckCtx, adapter, siteConfig, torrentData)
			recheckCancel()
			if dup && mTID > 0 {
				tidShown := fmt.Sprintf("%d", mTID)
				if e.logger != nil {
					e.logger.Info("music upload timeout but site recheck hit",
						zap.String("site", in.TargetSite), zap.String("tid", tidShown), zap.Error(upErr))
				}
				timeoutRecheckMsg = "上传超时但站方复查确认已发布（tid=" + tidShown + "）: " + dupMsg
				resp = &model.PublishResponse{
					Success:    true,
					IsExisting: true,
					TorrentID:  tidShown,
					DetailURL:  detailURLOf(siteConfig, tidShown),
				}
				upErr = nil
			}
		}
		if upErr != nil {
			if isUploadTimeoutErr(upErr) {
				return failRec("failed", fmt.Sprintf("上传超时且站方复查不可用（%v）——超时≠失败，请上站核验是否已发布后再决定重试", upErr))
			}
			return failRec("failed", fmt.Sprintf("上传失败: %v", upErr))
		}
	}
	if !resp.Success {
		if resp.ErrorMessage != "" {
			return failRec("failed", resp.ErrorMessage)
		}
		return failRec("failed", "上传未成功: "+resp.DetailURL)
	}

	// §59.159 同款：已存在分流
	uploadedStatus := "uploaded"
	if resp.IsExisting {
		if resp.ExistingID == "" && resp.TorrentID == "" {
			msg2 := "站上已有同种（未定位站内 ID，未推种——辅种业务范畴）"
			e.recordResult(ctx, in, meta, rv, "existing", msg2, "", "")
			return &ExecuteResult{Status: "existing", Message: msg2, TargetTorrentURL: resp.DetailURL}
		}
		uploadedStatus = "uploaded_existing"
		if resp.ExistingID != "" && resp.TorrentID == "" {
			resp.TorrentID = resp.ExistingID
		}
	}

	result := &ExecuteResult{
		Status:           uploadedStatus,
		Upload:           resp,
		TargetTorrentURL: resp.DetailURL,
	}
	if timeoutRecheckMsg != "" {
		result.Message = timeoutRecheckMsg
	}
	if e.pipe.pusher != nil {
		pushReq := &pusher.PushRequest{
			ClientUID: rv.ClientUID,
			SiteName:  in.TargetSite,
			TorrentID: resp.TorrentID,
			InfoHash:  in.InfoHash,
			Title:     meta.Title,
			SavePath:  rv.SavePath,
		}
		if pr := e.pipe.pusher.Push(ctx, pushReq); pr != nil {
			mergeMsg := func(m string) string {
				if timeoutRecheckMsg != "" {
					return timeoutRecheckMsg + "；" + m
				}
				return m
			}
			switch {
			case pr.Success:
				if uploadedStatus == "uploaded_existing" {
					result.Status = "pushed_existing"
				} else {
					result.Status = "pushed"
				}
			case pr.AlreadyExist:
				result.Status = "pushed_existing"
				result.Message = mergeMsg("下载器已有该种子（已做种）")
			default:
				if pr.Error != nil {
					result.Message = mergeMsg("加种失败: " + pr.Error.Error())
				}
			}
		}
	}

	seeded := result.Status == "pushed" || result.Status == "pushed_existing"
	e.recordResultFullWithLogs(ctx, in, meta, rv, result.Status,
		result.Message, resp.TorrentID, result.TargetTorrentURL, seeded, piecesHash, "")
	return result
}
