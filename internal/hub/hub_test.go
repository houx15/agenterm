package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestProtocolMarshalOutputMessage(t *testing.T) {
	msg := OutputMessage{
		Type:   "output",
		Window: "@0",
		Text:   "hello world",
		Class:  "normal",
		ID:     "msg-1",
		Ts:     1234567890,
		Actions: []ActionMessage{
			{Label: "Yes", Keys: "y\n"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded OutputMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("type mismatch: got %q, want %q", decoded.Type, msg.Type)
	}
	if decoded.Window != msg.Window {
		t.Errorf("window mismatch: got %q, want %q", decoded.Window, msg.Window)
	}
	if decoded.Text != msg.Text {
		t.Errorf("text mismatch: got %q, want %q", decoded.Text, msg.Text)
	}
	if decoded.Class != msg.Class {
		t.Errorf("class mismatch: got %q, want %q", decoded.Class, msg.Class)
	}
	if len(decoded.Actions) != 1 || decoded.Actions[0].Label != "Yes" {
		t.Errorf("actions mismatch: got %v", decoded.Actions)
	}
}

func TestProtocolMarshalClientMessage(t *testing.T) {
	msg := ClientMessage{
		Type:      "input",
		SessionID: "demo-session",
		Window:    "@0",
		Keys:      "ls -la\n",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ClientMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Type != "input" {
		t.Errorf("type mismatch: got %q", decoded.Type)
	}
	if decoded.Window != "@0" {
		t.Errorf("window mismatch: got %q", decoded.Window)
	}
	if decoded.SessionID != "demo-session" {
		t.Errorf("session mismatch: got %q", decoded.SessionID)
	}
	if decoded.Keys != "ls -la\n" {
		t.Errorf("keys mismatch: got %q", decoded.Keys)
	}
}

func TestTerminalInputRoutesSessionID(t *testing.T) {
	h := New("token", nil)
	calls := 0
	h.SetOnTerminalInputWithSession(func(sessionID string, windowID string, keys string) {
		calls++
		if sessionID != "s-1" || windowID != "@9" || keys != "pwd\n" {
			t.Fatalf("unexpected callback payload: session=%q window=%q keys=%q", sessionID, windowID, keys)
		}
	})

	h.handleTerminalInput("s-1", "@9", "pwd\n")
	if calls != 1 {
		t.Fatalf("expected callback to be called once, got %d", calls)
	}
}

func TestBroadcastToClientsRespectsSessionSubscription(t *testing.T) {
	h := New("token", nil)

	clientA := &Client{
		id:            "a",
		send:          make(chan []byte, 1),
		subscribeAll:  false,
		subscriptions: map[string]struct{}{"s-1": {}},
	}
	clientB := &Client{
		id:            "b",
		send:          make(chan []byte, 1),
		subscribeAll:  false,
		subscriptions: map[string]struct{}{"s-2": {}},
	}
	clientAll := &Client{
		id:            "all",
		send:          make(chan []byte, 1),
		subscribeAll:  true,
		subscriptions: map[string]struct{}{},
	}

	h.clients = map[string]*Client{
		clientA.id:   clientA,
		clientB.id:   clientB,
		clientAll.id: clientAll,
	}

	h.broadcastToClients(hubBroadcast{data: []byte(`{"type":"output"}`), sessionID: "s-1"})

	select {
	case <-clientA.send:
	default:
		t.Fatal("expected clientA to receive message for s-1")
	}
	select {
	case <-clientAll.send:
	default:
		t.Fatal("expected subscribe-all client to receive message")
	}
	select {
	case <-clientB.send:
		t.Fatal("did not expect clientB to receive message for s-1")
	default:
	}
}

func TestTokenAuthentication(t *testing.T) {
	validToken := "secret-token-123"

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"valid token", validToken, http.StatusSwitchingProtocols},
		{"invalid token", "wrong-token", http.StatusUnauthorized},
		{"missing token", "", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := New(validToken, nil)

			ctx, cancel := context.WithCancel(context.Background())
			go hub.Run(ctx)
			defer cancel()

			server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
			defer server.Close()

			url := fmt.Sprintf("ws://%s/ws", server.URL[7:])
			if tt.token != "" {
				url = fmt.Sprintf("%s?token=%s", url, tt.token)
			}

			dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
			conn, resp, err := websocket.Dial(dialCtx, url, nil)
			dialCancel()

			if resp != nil && resp.StatusCode != tt.wantStatus {
				t.Errorf("status code mismatch: got %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusSwitchingProtocols {
				if err != nil {
					t.Fatalf("expected successful connection, got error: %v", err)
				}
				conn.Close(websocket.StatusNormalClosure, "")
			} else if conn != nil {
				conn.Close(websocket.StatusNormalClosure, "")
			}
		})
	}
}

func TestClientLifecycle(t *testing.T) {
	token := "test-token"
	var inputReceived []string
	var inputMu sync.Mutex

	hub := New(token, func(windowID, keys string) {
		inputMu.Lock()
		inputReceived = append(inputReceived, fmt.Sprintf("%s:%s", windowID, keys))
		inputMu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}

	url := fmt.Sprintf("ws://%s/ws?token=%s", server.URL[7:], token)
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
	conn, _, err := websocket.Dial(dialCtx, url, nil)
	dialCancel()
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	waitForClientCount(t, hub, 1, 1*time.Second)

	inputMsg := ClientMessage{Type: "input", Window: "@0", Keys: "test\n"}
	data, _ := json.Marshal(inputMsg)
	writeCtx, writeCancel := context.WithTimeout(context.Background(), 1*time.Second)
	err = conn.Write(writeCtx, websocket.MessageText, data)
	writeCancel()
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	inputMu.Lock()
	if len(inputReceived) != 1 || inputReceived[0] != "@0:test\n" {
		t.Errorf("input not received correctly: %v", inputReceived)
	}
	inputMu.Unlock()

	conn.Close(websocket.StatusNormalClosure, "")

	waitForClientCount(t, hub, 0, 1*time.Second)
}

func TestBroadcastFanOut(t *testing.T) {
	token := "test-token"
	hub := New(token, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	url := fmt.Sprintf("ws://%s/ws?token=%s", server.URL[7:], token)

	var clients []*websocket.Conn
	for i := 0; i < 2; i++ {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
		conn, _, err := websocket.Dial(dialCtx, url, nil)
		dialCancel()
		if err != nil {
			t.Fatalf("failed to connect client %d: %v", i, err)
		}
		clients = append(clients, conn)
	}

	waitForClientCount(t, hub, 2, 1*time.Second)

	hub.SetBatchEnabled(false)
	hub.BroadcastOutput(OutputMessage{
		Type:   "output",
		Window: "@0",
		Text:   "broadcast test",
		Class:  "normal",
		Ts:     time.Now().Unix(),
	})

	for i, conn := range clients {
		readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, data, err := conn.Read(readCtx)
		readCancel()
		if err != nil {
			t.Fatalf("client %d failed to receive initial windows message: %v", i, err)
		}

		var baseMsg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &baseMsg); err != nil {
			t.Fatalf("client %d failed to unmarshal base message: %v", i, err)
		}
		if baseMsg.Type != "windows" {
			t.Fatalf("client %d expected initial windows message, got type: %s", i, baseMsg.Type)
		}

		readCtx, readCancel = context.WithTimeout(context.Background(), 2*time.Second)
		_, data, err = conn.Read(readCtx)
		readCancel()
		if err != nil {
			t.Fatalf("client %d failed to receive output message: %v", i, err)
		}

		var msg OutputMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("client %d failed to unmarshal: %v", i, err)
		}

		if msg.Text != "broadcast test" {
			t.Errorf("client %d received wrong text: %q", i, msg.Text)
		}
	}

	for _, conn := range clients {
		conn.Close(websocket.StatusNormalClosure, "")
	}
}

func TestRateLimiting(t *testing.T) {
	token := "test-token"
	hub := New(token, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	url := fmt.Sprintf("ws://%s/ws?token=%s", server.URL[7:], token)
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
	conn, _, err := websocket.Dial(dialCtx, url, nil)
	dialCancel()
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForClientCount(t, hub, 1, 1*time.Second)

	readCtx, readCancel := context.WithTimeout(context.Background(), 1*time.Second)
	_, _, err = conn.Read(readCtx)
	readCancel()
	if err != nil {
		t.Fatalf("failed to receive initial windows message: %v", err)
	}

	hub.SetBatchEnabled(true)
	for i := 0; i < 5; i++ {
		hub.BroadcastOutput(OutputMessage{
			Type:   "output",
			Window: "@0",
			Text:   fmt.Sprintf("msg%d ", i),
			Class:  "normal",
			Ts:     time.Now().Unix(),
		})
	}

	time.Sleep(200 * time.Millisecond)

	readCtx, readCancel = context.WithTimeout(context.Background(), 2*time.Second)
	_, data, err := conn.Read(readCtx)
	readCancel()
	if err != nil {
		t.Fatalf("failed to receive message: %v", err)
	}

	var msg OutputMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !strings.Contains(msg.Text, "msg0") {
		t.Errorf("batched message should contain msg0, got: %q", msg.Text)
	}
}

func TestRateLimiterDirect(t *testing.T) {
	var received []OutputMessage
	var mu sync.Mutex

	limiter := NewRateLimiter(50*time.Millisecond, func(windowID string, msg OutputMessage) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})

	for i := 0; i < 3; i++ {
		limiter.Add(OutputMessage{
			Type:   "output",
			Window: "@0",
			Text:   fmt.Sprintf("text%d ", i),
			Class:  "normal",
			Ts:     int64(i + 1),
		})
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(received) != 1 {
		t.Errorf("expected 1 batched message, got %d", len(received))
	}
	if len(received) > 0 && !strings.Contains(received[0].Text, "text0") {
		t.Errorf("batched message should contain all texts, got: %q", received[0].Text)
	}
	mu.Unlock()
}

func TestConnectionBeforeRun(t *testing.T) {
	token := "test-token"
	hub := New(token, nil)

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	url := fmt.Sprintf("ws://%s/ws?token=%s", server.URL[7:], token)

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
	conn, _, err := websocket.Dial(dialCtx, url, nil)
	dialCancel()
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	time.Sleep(100 * time.Millisecond)

	readCtx, readCancel := context.WithTimeout(context.Background(), 1*time.Second)
	_, data, err := conn.Read(readCtx)
	readCancel()
	if err != nil {
		t.Fatalf("failed to receive initial message: %v", err)
	}

	var msg WindowsMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if msg.Type != "windows" {
		t.Errorf("expected windows message, got type: %s", msg.Type)
	}

	conn.Close(websocket.StatusNormalClosure, "")
	cancel()
}

func TestInitialEmptyWindowsMessage(t *testing.T) {
	token := "test-token"
	hub := New(token, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	url := fmt.Sprintf("ws://%s/ws?token=%s", server.URL[7:], token)
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
	conn, _, err := websocket.Dial(dialCtx, url, nil)
	dialCancel()
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	readCtx, readCancel := context.WithTimeout(context.Background(), 1*time.Second)
	_, data, err := conn.Read(readCtx)
	readCancel()
	if err != nil {
		t.Fatalf("failed to receive initial windows message: %v", err)
	}

	var msg WindowsMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if msg.Type != "windows" {
		t.Errorf("expected windows message, got type: %s", msg.Type)
	}
	if len(msg.List) != 0 {
		t.Errorf("expected empty list, got %d items", len(msg.List))
	}
}

func TestHighClientCountShutdown(t *testing.T) {
	token := "test-token"
	hub := New(token, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	url := fmt.Sprintf("ws://%s/ws?token=%s", server.URL[7:], token)

	numClients := 20
	var conns []*websocket.Conn
	for i := 0; i < numClients; i++ {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
		conn, _, err := websocket.Dial(dialCtx, url, nil)
		dialCancel()
		if err != nil {
			t.Fatalf("failed to connect client %d: %v", i, err)
		}
		conns = append(conns, conn)
	}

	waitForClientCount(t, hub, numClients, 2*time.Second)

	cancel()
	time.Sleep(200 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after shutdown, got %d", hub.ClientCount())
	}

	for _, conn := range conns {
		conn.Close(websocket.StatusNormalClosure, "")
	}
}

func waitForClientCount(t *testing.T, hub *Hub, expected int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hub.ClientCount() == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if hub.ClientCount() != expected {
		t.Errorf("expected %d clients, got %d", expected, hub.ClientCount())
	}
}
