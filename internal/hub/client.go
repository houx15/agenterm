package hub

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

type Client struct {
	id            string
	conn          *websocket.Conn
	send          chan []byte
	hub           *Hub
	subMu         sync.RWMutex
	subscribeAll  bool
	subscriptions map[string]struct{}
}

func newClient(conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		id:            generateID(),
		conn:          conn,
		send:          make(chan []byte, 256),
		hub:           hub,
		subscribeAll:  true,
		subscriptions: make(map[string]struct{}),
	}
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.unregisterClient(c)
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.conn.SetReadLimit(32768)

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				log.Printf("client %s read error: %v", c.id, err)
			}
			return
		}

		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("client %s invalid message: %v", c.id, err)
			c.hub.SendError(c, "invalid message format")
			continue
		}

		switch msg.Type {
		case "input":
			if msg.Window != "" && msg.Keys != "" {
				c.hub.handleInput(msg.SessionID, msg.Window, msg.Keys)
			}
		case "terminal_input":
			if msg.Window != "" && msg.Keys != "" {
				c.hub.handleTerminalInput(msg.SessionID, msg.Window, msg.Keys)
			}
		case "terminal_resize":
			if msg.Window != "" && msg.Cols > 0 && msg.Rows > 0 {
				c.hub.handleTerminalResize(msg.SessionID, msg.Window, msg.Cols, msg.Rows)
			}
		case "subscribe":
			c.subscribe(msg.SessionID)
		case "new_session":
			c.hub.handleNewSession(msg.SessionID, msg.Name)
		case "new_window":
			c.hub.handleNewWindow(msg.SessionID, msg.Name)
		case "kill_window":
			c.hub.handleKillWindow(msg.SessionID, msg.Window)
		default:
			c.hub.SendError(c, "unknown message type: "+msg.Type)
		}
	}
}

func (c *Client) subscribe(sessionID string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	if sessionID == "" {
		c.subscribeAll = true
		c.subscriptions = make(map[string]struct{})
		return
	}
	c.subscribeAll = false
	c.subscriptions[sessionID] = struct{}{}
}

func (c *Client) wantsSession(sessionID string) bool {
	if sessionID == "" {
		return true
	}
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	if c.subscribeAll {
		return true
	}
	_, ok := c.subscriptions[sessionID]
	return ok
}

func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case <-ctx.Done():
			c.conn.Close(websocket.StatusNormalClosure, "")
			return
		case <-ticker.C:
			err := c.conn.Ping(ctx)
			if err != nil {
				return
			}
		case msg, ok := <-c.send:
			if !ok {
				c.conn.Close(websocket.StatusNormalClosure, "")
				return
			}

			err := c.conn.Write(ctx, websocket.MessageText, msg)
			if err != nil {
				return
			}
		}
	}
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}
