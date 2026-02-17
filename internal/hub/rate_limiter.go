package hub

import (
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.Mutex
	pending  map[string]*pendingOutput
	interval time.Duration
	onFlush  func(key string, msg OutputMessage)
}

type pendingOutput struct {
	sessionID string
	windowID  string
	texts     []string
	class     string
	ids       []string
	ts        int64
	actions   []ActionMessage
	timer     *time.Timer
}

func NewRateLimiter(interval time.Duration, onFlush func(string, OutputMessage)) *RateLimiter {
	return &RateLimiter{
		pending:  make(map[string]*pendingOutput),
		interval: interval,
		onFlush:  onFlush,
	}
}

func (r *RateLimiter) Add(msg OutputMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := rateLimitKey(msg.SessionID, msg.Window)
	p, exists := r.pending[key]
	if !exists {
		p = &pendingOutput{
			sessionID: msg.SessionID,
			windowID:  msg.Window,
			class:     msg.Class,
			ts:        msg.Ts,
		}
		r.pending[key] = p
	}

	p.texts = append(p.texts, msg.Text)
	if msg.ID != "" {
		p.ids = append(p.ids, msg.ID)
	}
	if len(msg.Actions) > 0 {
		p.actions = append(p.actions, msg.Actions...)
	}

	if p.timer == nil {
		p.timer = time.AfterFunc(r.interval, func() {
			r.flushWindow(key)
		})
	}
}

func (r *RateLimiter) flushWindow(key string) {
	r.mu.Lock()
	p, exists := r.pending[key]
	if !exists {
		r.mu.Unlock()
		return
	}
	delete(r.pending, key)
	r.mu.Unlock()

	if r.onFlush != nil && len(p.texts) > 0 {
		msg := OutputMessage{
			Type:      "output",
			SessionID: p.sessionID,
			Window:    p.windowID,
			Text:      strings.Join(p.texts, ""),
			Class:     p.class,
			ID:        strings.Join(p.ids, ","),
			Ts:        p.ts,
			Actions:   p.actions,
		}
		r.onFlush(key, msg)
	}
}

func (r *RateLimiter) FlushAll() {
	r.mu.Lock()
	windows := make([]string, 0, len(r.pending))
	for w := range r.pending {
		windows = append(windows, w)
	}
	r.mu.Unlock()

	for _, w := range windows {
		r.flushWindow(w)
	}
}

func rateLimitKey(sessionID string, windowID string) string {
	return sessionID + "::" + windowID
}
