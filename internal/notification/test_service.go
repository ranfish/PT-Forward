package notification

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type TestService struct {
	channel *model.NotificationChannel
	logger  *zap.Logger
	client  *http.Client
}

func NewTestService(ch *model.NotificationChannel, logger *zap.Logger) *TestService {
	return &TestService{
		channel: ch,
		logger:  logger,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *TestService) SendTest(ctx context.Context, msg model.FormattedMessage) (bool, string) {
	svc := &Service{db: nil, logger: t.logger, client: t.client}
	success, errMsg := svc.sendToChannel(ctx, t.channel, msg)
	if success {
		return true, "发送成功"
	}
	return false, fmt.Sprintf("发送失败: %s", errMsg)
}
