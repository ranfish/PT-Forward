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
				close(client.send)
				_ = client.conn.Close()
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
				close(client.send)
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
					close(c.send)
					h.mu.Unlock()
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

	client := &Client{
		hub:      s.hub,
		conn:     conn,
		send:     make(chan *WSMessage, 64),
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

	client.send <- NewWSMessage("server.connected", map[string]string{
		"serverTime": time.Now().UTC().Format(time.RFC3339),
	})
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
			select {
			case c.send <- NewWSMessage("pong", nil):
			default:
			}
		case "subscribe":
			c.mu.Lock()
			for _, ch := range msg.Channels {
				c.channels[ch] = true
			}
			c.mu.Unlock()
			select {
			case c.send <- NewWSMessage("subscribed", map[string]interface{}{"channels": msg.Channels}):
			default:
			}
		case "unsubscribe":
			c.mu.Lock()
			for _, ch := range msg.Channels {
				delete(c.channels, ch)
			}
			c.mu.Unlock()
			select {
			case c.send <- NewWSMessage("unsubscribed", map[string]interface{}{"channels": msg.Channels}):
			default:
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
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
