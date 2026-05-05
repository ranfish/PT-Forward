package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	stop := make(chan struct{})
	defer close(stop)
	go hub.Run(stop)

	hub.Broadcast(NewWSMessage("test.event", map[string]string{"key": "value"}))

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestHub_ClientReceive(t *testing.T) {
	hub := NewHub()
	stop := make(chan struct{})
	defer close(stop)
	go hub.Run(stop)

	authManager := setupTestEnv(t).authManager
	pair, _ := authManager.IssueTokenPair()

	wsHandler := NewWSHandler(hub, authManager, []string{"*"})

	server := httptest.NewServer(wsHandler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=" + pair.AccessToken
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var wsMsg WSMessage
	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if wsMsg.Type != "server.connected" {
		t.Errorf("expected server.connected, got %s", wsMsg.Type)
	}

	hub.Broadcast(NewWSMessage("task.created", map[string]string{"taskId": "test-1"}))
	time.Sleep(50 * time.Millisecond)

	_, msg2, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read broadcast: %v", err)
	}
	var wsMsg2 WSMessage
	if err := json.Unmarshal(msg2, &wsMsg2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if wsMsg2.Type != "task.created" {
		t.Errorf("expected task.created, got %s", wsMsg2.Type)
	}
}

func TestHub_Subscribe(t *testing.T) {
	hub := NewHub()
	stop := make(chan struct{})
	defer close(stop)
	go hub.Run(stop)

	authManager := setupTestEnv(t).authManager
	pair, _ := authManager.IssueTokenPair()

	wsHandler := NewWSHandler(hub, authManager, []string{"*"})
	server := httptest.NewServer(wsHandler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=" + pair.AccessToken
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	time.Sleep(50 * time.Millisecond)

	if err := conn.WriteJSON(map[string]interface{}{
		"type":     "client.subscribe",
		"channels": []string{"dashboard"},
	}); err != nil {
		t.Fatalf("write json: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := conn.WriteJSON(map[string]interface{}{
		"type": "client.ping",
	}); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	var foundPong bool
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if wsMsg.Type == "server.pong" {
			foundPong = true
			break
		}
	}
	if !foundPong {
		t.Error("expected server.pong response")
	}
}

func TestHub_UnauthorizedConnection(t *testing.T) {
	hub := NewHub()
	authManager := setupTestEnv(t).authManager

	wsHandler := NewWSHandler(hub, authManager, []string{"*"})
	server := httptest.NewServer(wsHandler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=invalid"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Error("expected error for invalid token")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestNewWSMessage(t *testing.T) {
	msg := NewWSMessage("test.type", map[string]string{"foo": "bar"})
	if msg.Type != "test.type" {
		t.Errorf("expected test.type, got %s", msg.Type)
	}
	if msg.ID == "" {
		t.Error("expected non-empty ID")
	}
	if msg.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}

	var payload map[string]string
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["foo"] != "bar" {
		t.Errorf("expected foo=bar, got %v", payload)
	}
}

func TestMatchChannel(t *testing.T) {
	tests := []struct {
		channel   string
		eventType string
		expected  bool
	}{
		{"task", "task.created", true},
		{"task", "task.progress", true},
		{"task", "dashboard.stats", false},
		{"dashboard", "dashboard.stats", true},
		{"seeding", "seeding.torrent.evaluated", true},
	}
	for _, tt := range tests {
		result := matchChannel(tt.channel, tt.eventType)
		if result != tt.expected {
			t.Errorf("matchChannel(%q, %q) = %v, expected %v", tt.channel, tt.eventType, result, tt.expected)
		}
	}
}
