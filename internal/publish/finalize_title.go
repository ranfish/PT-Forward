package publish

import (
	"context"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// FinalizeTitle 标题终态链共享单点（§59.514 附八·用户定案）。
//
// = PTGen 第四源 + .torrent 仲裁 + 归一 + v1.05 重组——发布链 / t0
// 获取链 / recompute 三消费方结构性同源（列=发布不靠纪律靠结构：
// 用户 Tab1 看到的=将要发的；显示错=重组链真错→上报开发者修链）。
//
// torrentData 空（isLocal=false/ExportTorrent 失败）时仲裁降级——
// 与发布链同款降级语义（发布时 ExportTorrent 失败=发布中止·获取时
// 失败=列降级落库·发布时再终态化——诚实分层）。
//
// 返回 (title, notes)：title=重组标题（空=四层全灭·调用方自处）；
// notes=仲裁/归一明细（落 reassembled_notes 列——Tab1 tooltip/
// DryRun 可观测）。
func (e *PublishExecutor) FinalizeTitle(ctx context.Context, meta *model.TorrentMetadata, tp *titleparser.TechProfile, torrentData []byte) (string, []string) {
	if meta == nil || tp == nil {
		return "", nil
	}
	e.mergePTGenSource(ctx, tp, meta)
	notes := e.arbitrateWithTorrent(ctx, meta, tp, torrentData)
	e.normalizeSeasonPackOf(meta, tp)
	rt := titleparser.ReassembleFromTechProfile(*tp, titleparser.V105TitleFormat())
	return strings.TrimSpace(rt), notes
}
