package model

// §56.23 决策 1 — ExistingStrategy: 已存在种子的处理策略。
type ExistingStrategy string

const (
	ExistingSkip   ExistingStrategy = "skip"   // 跳过（默认）
	ExistingUpdate ExistingStrategy = "update" // 自动更新（洗版）
	ExistingForce  ExistingStrategy = "force"  // 强制标记成功
)

// §56.23 决策 2 — EditRequest: 编辑提交请求。
// EditForm 已在 torrent.go 中定义（扩展加 Category/ExistingDesc 字段）。
type EditRequest struct {
	TorrentID  string            `json:"torrent_id"`
	FormFields map[string]string `json:"form_fields"`
	Cookie     string            `json:"-"`  // 站点 cookie（不序列化）
	Referer    string            `json:"-"`  // Referer URL
	BaseURL    string            `json:"-"`  // §56.23: 目标站 base URL
	APIKey     string            `json:"-"`  // §56.23: API 站认证 key
}

// ParseExistingStrategy 解析策略字符串，无效值返回默认 skip。
func ParseExistingStrategy(s string) ExistingStrategy {
	switch ExistingStrategy(s) {
	case ExistingSkip, ExistingUpdate, ExistingForce:
		return ExistingStrategy(s)
	default:
		return ExistingSkip
	}
}
