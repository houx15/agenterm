package hub

import (
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	mu         sync.Mutex
	pending    map[string]*pendingOutput
	interval   time.Duration
	onFlush    func(windowID string, msg OutputMessage)
	flushTimer *time.Timer
}

type pendingOutput struct {
	texts   []string
	class   string
	ids     []string
	ts      int64
	actions []ActionMessage
	timer   *time.Timer
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

	windowID := msg.Window
	p, exists := r.pending[windowID]
	if !exists {
		p = &pendingOutput{
			class: msg.Class,
		}
		r.pending[windowID] = p
	}

	p.texts = append(p.texts, msg.Text)
	if msg.ID != "" {
		p.ids = append(p.ids, msg.ID)
	}
	if msg.Ts > p.ts {
		p.ts = msg.Ts
	}
	if len(msg.Actions) > 0 {
		p.actions = append(p.actions, msg.Actions...)
	}

	if p.timer == nil {
		p.timer = time.AfterFunc(r.interval, func() {
			r.flushWindow(windowID)
		})
	}
}

func (r *RateLimiter) flushWindow(windowID string) {
	r.mu.Lock()
	p, exists := r.pending[windowID]
	if !exists {
		r.mu.Unlock()
		return
	}
	delete(r.pending, windowID)
	r.mu.Unlock()

	if r.onFlush != nil && len(p.texts) > 0 {
		msg := OutputMessage{
			Type:    "output",
			Window:  windowID,
			Text:    strings.Join(p.texts, ""),
			Class:   p.class,
			ID:      strings.Join(p.ids, ","),
			Ts:      p.ts,
			Actions: p.actions,
		}
		r.onFlush(windowID, msg)
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
