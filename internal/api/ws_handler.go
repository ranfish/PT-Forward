package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/ranfish/pt-forward/internal/auth"
)

type WSMessage struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp string          `json:"timestamp"`
	ID        string          `json:"id"`
}

func NewWSMessage(eventType string, payload interface{}) *WSMessage {
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte("null")
	}
	return &WSMessage{
		Type:      eventType,
		Payload:   data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		ID:        uuid.New().String(),
	}
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan *WSMessage
	userID   string
	channels map[string]bool
	mu       sync.RWMutex
	// 18 刀1（v0.0.1671）: 生命周期信号——channel 关闭责任单一化（V-01
	// 根修）。原三处 Hub close(client.send) 与四处发送方无协调——
	// stale 驱逐只关 send 不关 conn·readPump ping 一帧即
	// panic: send on closed channel（无 recover 杀进程·已鉴权客户端
	// 可稳定触发=远程 DoS·离线复现实锤）。新纪律：
	//   done 由 Hub 唯一关闭（shutdown 幂等）·send 永不被 close·
	//   所有发送方带 <-c.done 分支退出。
	done     chan struct{}
	doneOnce sync.Once
}

// shutdown 18 刀1: 幂等关闭 done——所有发送方（readPump 三处/
// ServeHTTP）与 writePump 经 done 分支退出。Hub 是唯一 owner。
func (c *Client) shutdown() {
	c.doneOnce.Do(func() { close(c.done) })
}

// trySend 18 刀1: 发送方统一形态——send 满→丢弃（default）·
// done 关→false（调用方退出）。永不向已关 channel 发送（send 不再
// 被 close）。
// 18 刀1 附（v0.0.1671 flake 根修）: 原单层三路 select 在「done 已关
// 且 send 有空位」时两 case 同就绪——Go select 随机选中，退出信号
// 50% 不确定（锚×10 实测 flake）。两阶段形态：前置 done 检查（关→
// 恒 false 确定退出）→未关走三路 select（间隙关闭的良性竞态由 send
// 永不被关兜底——消息入死 channel 无害 GC）。
func (c *Client) trySend(msg *WSMessage) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- msg:
		return true
	case <-c.done:
		return false
	default:
		return true // 缓冲满丢弃（既有语义）
	}
}

func (c *Client) ShouldReceive(eventType string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.channels) == 0 {
		return true
	}

	for ch := range c.channels {
		if matchChannel(ch, eventType) {
			return true
		}
	}
	return false
}

func matchChannel(channel, eventType string) bool {
	prefix := channel + "."
	if len(eventType) >= len(prefix) && eventType[:len(prefix)] == prefix {
		return true
	}
	return eventType == channel
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan *WSMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *WSMessage, 256),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
	}
}

func (h *Hub) Run(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			h.mu.Lock()
			for client := range h.clients {
				// 18 刀1（v0.0.1671）: close(client.send) → shutdown()
				// ——send 永不被关（V-01 根修·done 单一 owner）
				client.shutdown()
				if client.conn != nil {
					// 18 刀1 附: nil 守卫（stale/stop 路径测试构造可无 conn）
					_ = client.conn.Close()
				}
			}
			h.clients = make(map[*Client]bool)
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				// 18 刀1: shutdown()（原 close(send) 不关 conn——配对缺失）
				client.shutdown()
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			var stale []*Client
			for client := range h.clients {
				if !client.ShouldReceive(message.Type) {
					continue
				}
				select {
				case client.send <- message:
				default:
					stale = append(stale, client)
				}
			}
			h.mu.RUnlock()
			for _, c := range stale {
				select {
				case h.unregister <- c:
				default:
					h.mu.Lock()
					delete(h.clients, c)
					// 18 刀1（v0.0.1671）: close(c.send) → shutdown()
					// +conn.Close()（原只关 send 不关 conn——readPump 存活
					// 发 ping 即 panic 的原案路径。conn.Close 让 readPump
					// 立即退出·done 让所有发送方退出）
					c.shutdown()
					h.mu.Unlock()
					if c.conn != nil {
						_ = c.conn.Close()
					}
					slog.Warn("ws: unregister channel full, directly evicted stale client")
				}
			}
		}
	}
}

func (h *Hub) Broadcast(message *WSMessage) {
	select {
	case h.broadcast <- message:
	default:
		slog.Warn("ws: broadcast channel full, message dropped", "type", message.Type, "id", message.ID)
	}
}

func (h *Hub) BroadcastWS(eventType string, payload interface{}) {
	h.Broadcast(NewWSMessage(eventType, payload))
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

var defaultUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return false
	},
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
	authWait       = 10 * time.Second
)

type WSHandler struct {
	hub         *Hub
	authManager *auth.AuthManager
	corsOrigins []string
	upgrader    websocket.Upgrader
}

func NewWSHandler(hub *Hub, authManager *auth.AuthManager, corsOrigins []string) *WSHandler {
	upgrader := defaultUpgrader
	switch {
	case len(corsOrigins) > 0 && corsOrigins[0] == "*":
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
	case len(corsOrigins) > 0:
		allowed := make(map[string]bool, len(corsOrigins))
		for _, o := range corsOrigins {
			allowed[o] = true
		}
		upgrader.CheckOrigin = func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			return allowed[origin]
		}
	default:
		upgrader.CheckOrigin = func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			host := r.Host
			if host == "" {
				host = r.URL.Host
			}
			u, err := url.Parse(origin)
			if err != nil {
				return false
			}
			return u.Host == host
		}
	}
	return &WSHandler{hub: hub, authManager: authManager, corsOrigins: corsOrigins, upgrader: upgrader}
}

func (s *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(authWait))
	_, rawMsg, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return
	}

	var authMsg struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rawMsg, &authMsg); err != nil || authMsg.Type != "auth" || authMsg.Token == "" {
		_ = conn.WriteJSON(NewWSMessage("error", map[string]string{"reason": "auth required"}))
		_ = conn.Close()
		return
	}

	claims, err := s.authManager.ValidateAccessToken(authMsg.Token)
	if err != nil {
		_ = conn.WriteJSON(NewWSMessage("error", map[string]string{"reason": "invalid token"}))
		_ = conn.Close()
		return
	}

	_ = conn.SetReadDeadline(time.Time{})

	// 18 刀1（v0.0.1671）: done 初始化——Client 构造必经（V-01 根修
	// 生命周期信号）。
	client := &Client{
		hub:      s.hub,
		conn:     conn,
		send:     make(chan *WSMessage, 64),
		done:     make(chan struct{}),
		userID:   claims.Sub,
		channels: make(map[string]bool),
	}
	select {
	case s.hub.register <- client:
	default:
		slog.Warn("ws: register channel full, closing new connection")
		_ = conn.Close()
		return
	}

	go client.writePump()
	go client.readPump()

	// 18 刀1: 原裸阻塞发送——Hub stop/unregister 先关 send 则 panic·
	// send 满且 pump 退出则永久阻塞。改 trySend（done 退出+满丢弃）。
	client.trySend(NewWSMessage("server.connected", map[string]string{
		"serverTime": time.Now().UTC().Format(time.RFC3339),
	}))
}

func (c *Client) readPump() {
	defer func() {
		select {
		case c.hub.unregister <- c:
		default:
			slog.Warn("ws: unregister channel full on readPump exit")
		}
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var raw json.RawMessage
		err := c.conn.ReadJSON(&raw)
		if err != nil {
			break
		}

		var msg struct {
			Type     string   `json:"type"`
			Channels []string `json:"channels"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "ping":
			// 18 刀1（v0.0.1671）: trySend——send 永不被关·done 退出
			if !c.trySend(NewWSMessage("pong", nil)) {
				return
			}
		case "subscribe":
			c.mu.Lock()
			for _, ch := range msg.Channels {
				c.channels[ch] = true
			}
			c.mu.Unlock()
			if !c.trySend(NewWSMessage("subscribed", map[string]interface{}{"channels": msg.Channels})) {
				return
			}
		case "unsubscribe":
			c.mu.Lock()
			for _, ch := range msg.Channels {
				delete(c.channels, ch)
			}
			c.mu.Unlock()
			if !c.trySend(NewWSMessage("unsubscribed", map[string]interface{}{"channels": msg.Channels})) {
				return
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// 18 刀1（v0.0.1671）: send 永不被关（Hub 收口为 done）——
				// 此分支不再可达·保留防御（未来若有人误关 send 优雅退出）。
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-c.done:
			// 18 刀1: Hub 关闭 done——退出（conn 由 Hub/readPump defer 关）
			return

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
