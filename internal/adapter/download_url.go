// Package adapter 下载 URL 构造公共单点（§59.159 用户定案：抽公共方法，
// 框架差异显式分族——消费方一律走 SiteAdapter.DownloadTorrent 接口多态，
// 禁止在 pusher/executor/handler 自拼下载 URL）。
//
// 框架端点形态权威表（8 adapter 实测盘点）：
//   NexusPHP : /download.php?id=N[&passkey=]  （signed 模式=POST 签名 URL；
//              DownloadURLTemplate 站点覆盖优先——adapter 内分支）
//   Generic  : 同 NP（本单点合并——原双副本 buildURL/buildGenericDownloadURL）
//   Unit3D   : /torrents/download/{id} 或 /api/torrent/download/{id}（RSSKey）
//   TNode    : /torrents.php?action=download&id=
//   Gazelle  : /ajax.php?action=torrent&id=（authkey/rsskey）
//   MTeam    : x-api-key JSON API → .torrent URL（全 API 族）
//   YemaPT/Rousi: 特殊流程
package adapter

import (
	"net/url"
	"strings"
)

// BuildNexusDownloadURL NP 族下载 URL 公共单点（NexusPHP/Generic adapter 共用；
// 双副本历史：buildURL[adapter_nexusphp] 与 buildGenericDownloadURL[adapter_generic]
// 语义相同实现分叉——§59.26 同型教训，收敛）。
func BuildNexusDownloadURL(base, torrentID, passkey string) string {
	if !strings.HasPrefix(base, "http") {
		base = "https://" + base
	}
	u := strings.TrimRight(base, "/") + "/download.php?id=" + url.QueryEscape(torrentID)
	if passkey != "" {
		u += "&passkey=" + url.QueryEscape(passkey)
	}
	return u
}
