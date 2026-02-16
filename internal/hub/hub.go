package hub

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"
)

const defaultBatchInterval = 100 * time.Millisecond

type Hub struct {
	clients      map[string]*Client
	register     chan *clientRegistration
	unregister   chan *Client
	broadcast    chan []byte
	onInput      func(windowID string, keys string)
	onNewWindow  func(name string)
	token        string
	mu           sync.RWMutex
	windows      []WindowInfo
	windowsMu    sync.RWMutex
	rateLimiter  *RateLimiter
	batchEnabled bool
	ctxWrap      *ctxWrapper
	running      atomic.Bool
}

type ctxWrapper struct {
	ctx context.Context
}

type clientRegistration struct {
	client         *Client
	initialWindows []byte
}

func New(token string, onInput func(string, string)) *Hub {
	h := &Hub{
		clients:      make(map[string]*Client),
		register:     make(chan *clientRegistration, 16),
		unregister:   make(chan *Client, 16),
		broadcast:    make(chan []byte, 256),
		onInput:      onInput,
		token:        token,
		batchEnabled: true,
		ctxWrap:      &ctxWrapper{ctx: context.Background()},
	}
	h.rateLimiter = NewRateLimiter(defaultBatchInterval, func(windowID string, msg OutputMessage) {
		h.sendBroadcast(msg)
	})
	return h
}

func (h *Hub) getContext() context.Context {
	if h.ctxWrap != nil {
		return h.ctxWrap.ctx
	}
	return context.Background()
}

func (h *Hub) Run(ctx context.Context) {
	h.ctxWrap = &ctxWrapper{ctx: ctx}
	h.running.Store(true)
	defer h.running.Store(false)

	for {
		select {
		case <-ctx.Done():
			h.rateLimiter.FlushAll()
			h.mu.Lock()
			for _, c := range h.clients {
				close(c.send)
			}
			h.clients = make(map[string]*Client)
			h.mu.Unlock()
			return

		case reg := <-h.register:
			h.mu.Lock()
			h.clients[reg.client.id] = reg.client
			h.mu.Unlock()
			if reg.initialWindows != nil {
				select {
				case reg.client.send <- reg.initialWindows:
				default:
				}
			}
			go reg.client.writePump(h.getContext())
			go reg.client.readPump(h.getContext())
			log.Printf("client connected: %s (total: %d)", reg.client.id, h.ClientCount())

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.id]; ok {
				delete(h.clients, client.id)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("client disconnected: %s (total: %d)", client.id, h.ClientCount())

		case data := <-h.broadcast:
			h.mu.RLock()
			for _, c := range h.clients {
				select {
				case c.send <- data:
				default:
					log.Printf("client %s send buffer full, dropping message", c.id)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" || token != h.token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("websocket accept error: %v", err)
		return
	}

	client := newClient(conn, h)

	h.windowsMu.RLock()
	windows := h.windows
	h.windowsMu.RUnlock()

	msg := WindowsMessage{Type: "windows", List: windows}
	initialWindows, _ := json.Marshal(msg)

	select {
	case h.register <- &clientRegistration{client: client, initialWindows: initialWindows}:
	default:
		log.Printf("hub not accepting connections")
		conn.Close(websocket.StatusTryAgainLater, "server busy")
		return
	}
}

func (h *Hub) BroadcastOutput(msg OutputMessage) {
	if h.batchEnabled && h.rateLimiter != nil {
		h.rateLimiter.Add(msg)
	} else {
		h.sendBroadcast(msg)
	}
}

func (h *Hub) sendBroadcast(msg OutputMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("error marshaling output message: %v", err)
		return
	}
	select {
	case h.broadcast <- data:
	default:
		log.Printf("broadcast channel full, dropping message")
	}
}

func (h *Hub) BroadcastWindows(windows []WindowInfo) {
	h.windowsMu.Lock()
	h.windows = windows
	h.windowsMu.Unlock()

	msg := WindowsMessage{Type: "windows", List: windows}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("error marshaling windows message: %v", err)
		return
	}
	select {
	case h.broadcast <- data:
	default:
		log.Printf("broadcast channel full, dropping windows message")
	}
}

func (h *Hub) BroadcastStatus(windowID string, status string) {
	msg := StatusMessage{Type: "status", Window: windowID, Status: status}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("error marshaling status message: %v", err)
		return
	}
	select {
	case h.broadcast <- data:
	default:
		log.Printf("broadcast channel full, dropping status message")
	}
}

func (h *Hub) SendError(client *Client, message string) {
	msg := ErrorMessage{Type: "error", Message: message}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("error marshaling error message: %v", err)
		return
	}
	select {
	case client.send <- data:
	default:
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) handleInput(windowID string, keys string) {
	if h.onInput != nil {
		h.onInput(windowID, keys)
	}
}

func (h *Hub) handleNewWindow(name string) {
	if h.onNewWindow != nil {
		h.onNewWindow(name)
	}
}

func (h *Hub) SetOnNewWindow(fn func(name string)) {
	h.onNewWindow = fn
}

func (h *Hub) SetBatchEnabled(enabled bool) {
	h.batchEnabled = enabled
}

func (h *Hub) FlushPendingOutput() {
	if h.rateLimiter != nil {
		h.rateLimiter.FlushAll()
	}
}

func (h *Hub) isRunning() bool {
	return h.running.Load()
}

func (h *Hub) unregisterClient(c *Client) {
	if !h.isRunning() {
		c.conn.Close(websocket.StatusNormalClosure, "")
		return
	}
	select {
	case h.unregister <- c:
	default:
		log.Printf("unregister channel full for client %s, forcing close", c.id)
		c.conn.Close(websocket.StatusNormalClosure, "")
	}
}
