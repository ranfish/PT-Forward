package publish

// §59.559 PR-C: Execute 上传/dedup/落库段抽取。
// 前置锚：execute_upload_anchor_test.go（fingerprintPiecesHash/dedupStatusSets）
// + executor_timeout_test.go（isUploadTimeoutErr）——拆分前后双绿。

import (
	"context"

	"fmt"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/pusher"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// executeUploadContext 上传段输入（Execute 主函数产物汇聚）。
type executeUploadContext struct {
	in           ExecuteInput
	meta         *model.TorrentMetadata
	rv           *ResourceView
	cfg          *model.PublishFormConfig
	form         map[string]string
	fb           *executeFormBuild
	torrentData  []byte
	publishTitle string
}

// executeUploadAndDedup §59.559 PR-C: 上传段（原 Execute :331-566 抽取）——
// 审核门+pubReq 组装+ExtraFields 注入+adapter 获取+dedup 三路+上传+超时复查+已存在分流。
// 失败经 (failStatus, failMsg) 返回（主函数 failRec 落库）·成功返回 (resp, timeoutRecheckMsg)。
func (e *PublishExecutor) executeUploadAndDedup(ctx context.Context, uc *executeUploadContext) (*model.PublishResponse, string, string, string) {
	in, meta, rv, cfg, form, fb := uc.in, uc.meta, uc.rv, uc.cfg, uc.form, uc.fb

	// §59.273: 审核门
	if !meta.Reviewed {
		return nil, "", "unreviewed", "种子未审核（reviewed=false）——请先在种子配置页完成预览审核"
	}
	if cfg.Anonymous {
		in.Anonymous = true
	}
	pubReq := &model.PublishRequest{
		TorrentData:    uc.torrentData,
		FormFields:     form,
		ExtraArtists:   fb.gpwArtists,
		SubtitleLangs:  fb.gpwSubs,
		Title:          uc.publishTitle,
		Subtitle:       meta.Subtitle,
		Description:    form[cfg.FormFields[model.FieldDomainDescription]],
		MediaInfo:      meta.MediaInfo,
		BDInfo:         meta.BDInfo,
		Screenshots:    parseScreenshotsCol(meta.Screenshots),
		IMDbLink:       meta.IMDbURL,
		DoubanLink:     meta.DoubanURL,
		Anonymous:      in.Anonymous,
		SourceSite:     meta.SiteName,
		SourceInfoHash: in.InfoHash,
		TargetSite:     in.TargetSite,
		ClientUID:      rv.ClientUID,
	}
	pubReq.TagArrayFields = fb.tagArrayFields
	// §59.451: 馒头 labels 位掩码
	if in.TargetSite == "馒头" {
		if bits := mteamLabelsBits(fb.tags); bits > 0 {
			if pubReq.ExtraFields == nil {
				pubReq.ExtraFields = map[string]string{}
			}
			pubReq.ExtraFields["labels"] = strconv.Itoa(bits)
		}
	}
	// §59.462 刀⑤: 海豹 ExtraFields 注入+gpw_image Rehost
	if in.TargetSite == "海豹" && len(fb.gpwExtra) > 0 {
		if pubReq.ExtraFields == nil {
			pubReq.ExtraFields = map[string]string{}
		}
		for k, v := range fb.gpwExtra {
			pubReq.ExtraFields[k] = v
		}
		if src := pubReq.ExtraFields["gpw_image"]; src != "" &&
			e.pipe != nil && e.pipe.imageHostMgr != nil {
			if r, rErr := e.pipe.imageHostMgr.Rehost(ctx, src); rErr == nil && r != nil && r.URL != "" {
				pubReq.ExtraFields["gpw_image"] = r.URL
				e.logger.Info("gpw image rehosted", zap.String("to", r.URL))
			} else {
				e.logger.Warn("gpw image rehost failed（降级保留原 URL）", zap.Error(rErr))
			}
		}
	}
	// §59.458: 肉丝 ExtraFields 搬运
	if in.TargetSite == "肉丝" {
		if pubReq.ExtraFields == nil {
			pubReq.ExtraFields = map[string]string{}
		}
		for _, k := range []string{"rs_genre", "rs_region", "rs_poster"} {
			if v := form[k]; v != "" {
				pubReq.ExtraFields[k] = v
			}
		}
	}
	// §59.459: 野马 ExtraFields 搬运
	if in.TargetSite == "野马" {
		if pubReq.ExtraFields == nil {
			pubReq.ExtraFields = map[string]string{}
		}
		for _, k := range []string{"yema_regions", "yema_translated", "yema_poster"} {
			if v := form[k]; v != "" {
				pubReq.ExtraFields[k] = v
			}
		}
	}

	adapter, aErr := e.pipe.siteProvider.GetAdapter(ctx, in.TargetSite)
	if aErr != nil {
		return nil, "", "failed", fmt.Sprintf("获取站点适配器失败: %v", aErr)
	}
	siteConfig, scErr := e.pipe.siteProvider.GetSiteConfig(ctx, in.TargetSite)
	if scErr != nil {
		return nil, "", "failed", fmt.Sprintf("获取站点配置失败: %v", scErr)
	}

	// dedup 双路（§59.166 本地记忆优先）
	piecesHash := fingerprintPiecesHash(uc.torrentData)
	if piecesHash != "" {
		var rec model.PublishResultRecord
		if err := e.db.WithContext(ctx).
			Where("target_site = ? AND pieces_hash = ? AND status IN ?", in.TargetSite, piecesHash,
				model.PublishSkipStatuses).
			Order("id DESC").First(&rec).Error; err == nil {
			tidShown := rec.TorrentID
			if tidShown == "" {
				if _, mHash, mTID := e.pipe.dedupByPiecesHashFull(ctx, adapter, siteConfig, uc.torrentData); mTID > 0 {
					_ = mHash
					tidShown = fmt.Sprintf("%d", mTID)
				}
			}
			msg := "本地记忆命中（站上已有同内容种）: " + piecesHash
			e.recordResult(ctx, in, meta, rv, "duplicate", msg, tidShown, detailURLOf(siteConfig, tidShown), piecesHash)
			return nil, "", "duplicate", msg
		}
	}
	// ② 站方 pieces-hash API
	if dup, dupMsg, mTID := e.pipe.dedupByPiecesHashFull(ctx, adapter, siteConfig, uc.torrentData); dup {
		msg := "目标站已存在同内容种子: " + dupMsg
		tidShown := ""
		if mTID > 0 {
			tidShown = fmt.Sprintf("%d", mTID)
		}
		e.recordResult(ctx, in, meta, rv, "duplicate", msg, tidShown, detailURLOf(siteConfig, tidShown), piecesHash)
		return nil, "", "duplicate", msg
	}
	// ③ 站方 check 预查重（§59.515 刀2 D1）
	if checker, ok := adapter.(hdareaPreChecker); ok {
		pc, pcErr := checker.CheckBeforeUpload(ctx, siteConfig, uc.torrentData)
		if pcErr != nil {
			e.logger.Warn("check 预查重失败（放行·409 兜底）", zap.String("site", in.TargetSite), zap.Error(pcErr))
		} else if pc != nil && !pc.CanUpload {
			if pc.IsBanned {
				msg := "站方禁发此资源: " + pc.Reason
				e.recordResult(ctx, in, meta, rv, "failed", msg, "", "", piecesHash)
				return nil, "", "failed", msg
			}
			msg := "站方 check 拦截: " + pc.Reason
			if pc.ExistingName != "" {
				msg += "（" + pc.ExistingName + "）"
			}
			e.recordResult(ctx, in, meta, rv, "duplicate", msg, pc.ExistingID, detailURLOf(siteConfig, pc.ExistingID), piecesHash)
			return nil, "", "duplicate", msg
		} else if pc != nil && len(pc.ReplaceableDead) > 0 {
			e.logger.Info("check 顶替断种（站方将自动删旧上架）",
				zap.String("site", in.TargetSite),
				zap.Strings("dead_tids", pc.ReplaceableDead))
		}
	}

	resp, upErr := adapter.UploadTorrent(ctx, siteConfig, pubReq)
	timeoutRecheckMsg := ""
	if upErr != nil {
		if isUploadTimeoutErr(upErr) {
			recheckCtx, recheckCancel := context.WithTimeout(context.WithoutCancel(ctx), 60*1e9)
			dup, dupMsg, mTID := e.pipe.dedupByPiecesHashFull(recheckCtx, adapter, siteConfig, uc.torrentData)
			recheckCancel()
			if dup && mTID > 0 {
				tidShown := fmt.Sprintf("%d", mTID)
				if e.logger != nil {
					e.logger.Info("upload timeout but site recheck hit (treat as existing)",
						zap.String("site", in.TargetSite),
						zap.String("tid", tidShown),
						zap.Error(upErr))
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
				return nil, "", "failed", fmt.Sprintf("上传超时且站方复查不可用（%v）——超时≠失败，请上站核验是否已发布后再决定重试", upErr)
			}
			return nil, "", "failed", fmt.Sprintf("上传失败: %v", upErr)
		}
	}
	if !resp.Success {
		if resp.ErrorMessage != "" {
			return nil, "", "failed", resp.ErrorMessage
		}
		return nil, "", "failed", "上传未成功: " + resp.DetailURL
	}

	// §59.159: 已存在分流
	if resp.IsExisting {
		if resp.ExistingID == "" && resp.TorrentID == "" {
			msg2 := "站上已有同种（未定位站内 ID，未推种——辅种业务范畴）"
			e.recordResult(ctx, in, meta, rv, "existing", msg2, "", "")
			return &model.PublishResponse{DetailURL: resp.DetailURL}, "", "__existing__", msg2
		}
		if resp.ExistingID != "" && resp.TorrentID == "" {
			resp.TorrentID = resp.ExistingID
		}
	}
	return resp, timeoutRecheckMsg, "", ""
}

// finalizeExecuteResult §59.559 PR-C: 落库返回段（原 Execute :567-631 抽取）——
// 已存在分流状态+⑨ 加种+⑩ 落库。
func (e *PublishExecutor) finalizeExecuteResult(ctx context.Context, uc *executeUploadContext,
	resp *model.PublishResponse, uploadedStatus, timeoutRecheckMsg string, preAudit *PreAuditResult,
	localFindings []LocalAuditFinding) *ExecuteResult {
	in, meta, rv := uc.in, uc.meta, uc.rv

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
			ClientUID:   rv.ClientUID,
			SiteName:    in.TargetSite,
			TorrentID:   resp.TorrentID,
			InfoHash:    in.InfoHash,
			Title:       meta.Title,
			SavePath:    rv.SavePath,
			TorrentData: resp.TorrentData,
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
	if len(localFindings) > 0 {
		result.Message = strings.TrimSpace(strings.TrimSuffix(result.Message, "；") +
			"；" + FormatLocalAudit(localFindings))
	}
	e.recordResultFullWithLogs(ctx, in, meta, rv, result.Status,
		joinNotes(paNoteOf(preAudit), result.Message),
		resp.TorrentID, result.TargetTorrentURL, seeded, fingerprintPiecesHash(uc.torrentData),
		SerializeLocalAudit(localFindings))
	return result
}
