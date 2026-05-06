package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	logger *zap.Logger
	client *http.Client
}

func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Service) Send(ctx context.Context, msg model.FormattedMessage) error {
	var channels []model.NotificationChannel
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Find(&channels).Error; err != nil {
		return notifyError(ErrNotifyChannel, "查询通知通道失败", err)
	}

	if len(channels) == 0 {
		return nil
	}

	healthyChannels := make([]*model.NotificationChannel, 0, len(channels))
	failoverGroups := make(map[string][]*model.NotificationChannel)

	for i := range channels {
		ch := &channels[i]
		if !s.matchEvent(ch, "all") && !s.matchEvent(ch, msg.Level) {
			continue
		}
		if s.inQuietHours(ch) {
			continue
		}
		if ch.Healthy {
			healthyChannels = append(healthyChannels, ch)
		}
		if ch.FailoverGroupID != "" {
			failoverGroups[ch.FailoverGroupID] = append(failoverGroups[ch.FailoverGroupID], ch)
		}
	}

	var lastErr error
	for _, ch := range healthyChannels {
		success, errMsg := s.sendToChannel(ctx, ch, msg)

		s.recordHistory(ctx, ch.ID, msg, success, errMsg)

		if success {
			s.resetFailures(ctx, ch)
		} else {
			lastErr = notifyError(ErrNotifyChannel, fmt.Sprintf("%s: %s", ch.Name, errMsg), nil)
			s.incrementFailures(ctx, ch)

			if ch.FailoverGroupID != "" {
				if fallbacks, ok := failoverGroups[ch.FailoverGroupID]; ok {
					for _, fb := range fallbacks {
						if fb.ID == ch.ID || !fb.Healthy {
							continue
						}
						fbOk, fbErr := s.sendToChannel(ctx, fb, msg)
						s.recordHistory(ctx, fb.ID, msg, fbOk, fbErr)
						if fbOk {
							s.logger.Info("故障转移到备用通道",
								zap.String("from", ch.Name),
								zap.String("to", fb.Name),
							)
							break
						}
					}
				}
			}
		}
	}

	return lastErr
}

func (s *Service) Dispatch(ctx context.Context, event string, msg model.FormattedMessage) error {
	var channels []model.NotificationChannel
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Find(&channels).Error; err != nil {
		return err
	}

	var lastErr error
	for i := range channels {
		ch := &channels[i]
		if !s.matchEvent(ch, event) {
			continue
		}
		if !ch.Healthy {
			continue
		}
		if s.inQuietHours(ch) {
			continue
		}

		success, errMsg := s.sendToChannel(ctx, ch, msg)
		s.recordHistory(ctx, ch.ID, msg, success, errMsg)

		if success {
			s.resetFailures(ctx, ch)
		} else {
			lastErr = notifyError(ErrNotifyChannel, fmt.Sprintf("%s: %s", ch.Name, errMsg), nil)
			s.incrementFailures(ctx, ch)
		}
	}

	return lastErr
}

func (s *Service) Test(ctx context.Context) error {
	msg := model.FormattedMessage{
		Title:   "PT-Forward 测试通知",
		Message: "如果你看到这条消息，说明通知通道配置正确。",
		Level:   "info",
	}
	return s.Send(ctx, msg)
}

func (s *Service) sendToChannel(ctx context.Context, ch *model.NotificationChannel, msg model.FormattedMessage) (bool, string) {
	timeout := s.channelTimeout(ch)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch ch.Type {
	case "telegram":
		return s.sendTelegram(ctx, ch, msg)
	case "bark":
		return s.sendBark(ctx, ch, msg)
	case "webhook":
		return s.sendWebhook(ctx, ch, msg)
	case "serverchan":
		return s.sendServerChan(ctx, ch, msg)
	case "dingtalk":
		return s.sendDingTalk(ctx, ch, msg)
	default:
		return false, "不支持的通知类型: " + ch.Type
	}
}

func (s *Service) channelTimeout(ch *model.NotificationChannel) time.Duration {
	if ch.TimeoutMs > 0 {
		return time.Duration(ch.TimeoutMs) * time.Millisecond
	}
	return 15 * time.Second
}

func (s *Service) sendTelegram(ctx context.Context, ch *model.NotificationChannel, msg model.FormattedMessage) (bool, string) {
	cfg := parseConfig(ch.Config)
	token := cfg["token"]
	chatID := cfg["chat_id"]
	if token == "" || chatID == "" {
		return false, "缺少 token 或 chat_id"
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	text := fmt.Sprintf("<b>%s</b>\n%s", escapeHTML(msg.Title), escapeHTML(msg.Message))
	body := fmt.Sprintf(`{"chat_id":"%s","text":"%s","parse_mode":"HTML"}`, chatID, text)

	return s.doHTTPPost(ctx, apiURL, "application/json", body)
}

func (s *Service) sendBark(ctx context.Context, ch *model.NotificationChannel, msg model.FormattedMessage) (bool, string) {
	cfg := parseConfig(ch.Config)
	serverURL := cfg["url"]
	deviceKey := cfg["device_key"]
	if serverURL == "" || deviceKey == "" {
		return false, "缺少 url 或 device_key"
	}

	apiURL := fmt.Sprintf("%s/%s/%s/%s", strings.TrimRight(serverURL, "/"), deviceKey, url.PathEscape(msg.Title), url.PathEscape(msg.Message))
	return s.doHTTPGet(ctx, apiURL)
}

func (s *Service) sendServerChan(ctx context.Context, ch *model.NotificationChannel, msg model.FormattedMessage) (bool, string) {
	cfg := parseConfig(ch.Config)
	sendKey := cfg["sendkey"]
	if sendKey == "" {
		return false, "缺少 sendkey"
	}

	apiURL := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", sendKey)
	body := fmt.Sprintf(`{"title":"%s","desp":"%s"}`, msg.Title, msg.Message)
	return s.doHTTPPost(ctx, apiURL, "application/json", body)
}

func (s *Service) sendDingTalk(ctx context.Context, ch *model.NotificationChannel, msg model.FormattedMessage) (bool, string) {
	cfg := parseConfig(ch.Config)
	webhook := cfg["webhook"]
	secret := cfg["secret"]
	if webhook == "" {
		return false, "缺少 webhook"
	}

	apiURL := webhook
	if secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := calcDingTalkSign(timestamp, secret)
		apiURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhook, timestamp, url.QueryEscape(sign))
	}

	body := fmt.Sprintf(`{"msgtype":"text","text":{"content":"%s\n%s"}}`, msg.Title, msg.Message)
	return s.doHTTPPost(ctx, apiURL, "application/json", body)
}

func (s *Service) sendWebhook(ctx context.Context, ch *model.NotificationChannel, msg model.FormattedMessage) (bool, string) {
	cfg := parseConfig(ch.Config)
	webhookURL := cfg["url"]
	if webhookURL == "" {
		return false, "缺少 url"
	}

	payload, _ := json.Marshal(map[string]string{
		"title":   msg.Title,
		"message": msg.Message,
		"level":   msg.Level,
	})
	return s.doHTTPPost(ctx, webhookURL, "application/json", string(payload))
}

func (s *Service) doHTTPPost(ctx context.Context, apiURL, contentType, body string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(body))
	if err != nil {
		return false, err.Error()
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "PT-Forward/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return false, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return true, ""
}

func (s *Service) doHTTPGet(ctx context.Context, apiURL string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return false, err.Error()
	}
	req.Header.Set("User-Agent", "PT-Forward/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return true, ""
}

func (s *Service) recordHistory(ctx context.Context, channelID uint, msg model.FormattedMessage, success bool, errMsg string) {
	history := &model.NotificationHistory{
		ChannelID: channelID,
		Event:     msg.Level,
		Level:     msg.Level,
		Title:     msg.Title,
		Body:      msg.Message,
		Success:   success,
		ErrorMsg:  errMsg,
	}
	if err := s.db.WithContext(ctx).Create(history).Error; err != nil {
		s.logger.Warn("record notification history failed", zap.Error(err))
	}
}

func (s *Service) resetFailures(ctx context.Context, ch *model.NotificationChannel) {
	s.db.WithContext(ctx).Model(ch).Updates(map[string]interface{}{
		"consecutive_failures": 0,
		"healthy":              true,
	})
}

func (s *Service) incrementFailures(ctx context.Context, ch *model.NotificationChannel) {
	newCount := ch.ConsecutiveFailures + 1
	updates := map[string]interface{}{
		"consecutive_failures": newCount,
	}
	if newCount >= ch.MaxErrorsPerHour && ch.MaxErrorsPerHour > 0 {
		updates["healthy"] = false
		s.logger.Warn("notification channel marked unhealthy", zap.String("name", ch.Name), zap.Int("failures", newCount))
	}
	s.db.WithContext(ctx).Model(ch).Updates(updates)
}

func (s *Service) matchEvent(ch *model.NotificationChannel, event string) bool {
	if ch.Events == "" || ch.Events == "*" {
		return true
	}
	events := strings.Split(ch.Events, ",")
	for _, e := range events {
		if strings.TrimSpace(e) == event {
			return true
		}
	}
	return false
}

func (s *Service) inQuietHours(ch *model.NotificationChannel) bool {
	if ch.QuietHoursStart == "" || ch.QuietHoursEnd == "" {
		return false
	}
	now := time.Now().Format("15:04")
	return now >= ch.QuietHoursStart && now <= ch.QuietHoursEnd
}

func parseConfig(configStr string) map[string]string {
	cfg := make(map[string]string)
	if configStr == "" {
		return cfg
	}
	_ = json.Unmarshal([]byte(configStr), &cfg)
	return cfg
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func calcDingTalkSign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
