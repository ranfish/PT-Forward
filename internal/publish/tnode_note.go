package publish

import (
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/model"
)

// bbcodeTagRe §59.554: BBCode 标签剥离（[quote]/[/quote]/[b] 等——
// 内文与换行保留）。朱雀 note 纯文本要求（用户定案：引用也只投纯文本）。
// [url=…]形态会丢链接文本——当前实证声明形态（quote 包裹纯文本）无此
// 场景·按需不预实施。
var bbcodeTagRe = regexp.MustCompile(`\[[^\]]+\]`)

func stripBBCodeTags(s string) string {
	if !strings.Contains(s, "[") {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(bbcodeTagRe.ReplaceAllString(s, ""))
}

// tnodeNoteOf §59.554 刀2: TNode note 三件套组装——纯文本声明 +
// IMDb/豆瓣链接行（全角冒号·零空格·站方发布页行格式定案）。三源独立
// 缺省各自跳行（空值矩阵：statement 空→仅链接行·链接空→缺行·全空→空串
// 不投）。数据源全在素材表（statement/im_db_url/douban_url·朱雀首两颗
// 实测全中）。
// §59.555 复测修正: 声明源改走 description.ComposeStatement（与通用链/
// tab2 展示同一单点·§59.542 追记十二消费时合成——官组行无致谢时动态
// 追加两行官方致谢「{站}官组作品…」「互珍互重，禁转PTT」）。首颗复测
// 用户抓漏：直接用原始 statement 列漏掉合成层。
func tnodeNoteOf(meta *model.TorrentMetadata) string {
	if meta == nil {
		return ""
	}
	var lines []string
	if stmt := stripBBCodeTags(description.ComposeStatement(meta.Statement, meta.SiteName, meta.Title)); stmt != "" {
		lines = append(lines, stmt)
	}
	if meta.IMDbURL != "" {
		lines = append(lines, "IMDb链接："+meta.IMDbURL)
	}
	if meta.DoubanURL != "" {
		lines = append(lines, "豆瓣链接："+meta.DoubanURL)
	}
	return strings.Join(lines, "\n")
}
