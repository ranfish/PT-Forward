package extract

// §56.13/§56.7 站点特殊 BBCode 处理（2b.7 实施时填充）。
//
// 当前 2b.3 仅占位。2b.7 将添加：
//   - PTer: <table> 整体丢弃（站点默认行为）
//   - HDSpace: URL 嵌套 img
//   - HDT: show link
//   - BHD: list 处理
//
// 通过 HTMLToBBCodeConverter.siteCode / siteNickname 在 lookupTagHandler 时
// 路由到站点特殊 handler（待 2b.7 接入）。
