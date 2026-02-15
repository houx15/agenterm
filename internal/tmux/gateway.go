package tmux

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

type Gateway struct {
	session string
	process *exec.Cmd
	stdin   io.WriteCloser
	events  chan Event
	done    chan struct{}
	windows map[string]*Window
	mu      sync.RWMutex
	wg      sync.WaitGroup
}

func New(session string) *Gateway {
	return &Gateway{
		session: session,
		events:  make(chan Event, 1000),
		done:    make(chan struct{}),
		windows: make(map[string]*Window),
	}
}

func (g *Gateway) Start(ctx context.Context) error {
	g.process = exec.CommandContext(ctx, "tmux", "-C", "attach-session", "-t", g.session)

	stdout, err := g.process.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	g.stdin, err = g.process.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := g.process.Start(); err != nil {
		if strings.Contains(err.Error(), "exit status") || strings.Contains(err.Error(), "no session") {
			return fmt.Errorf("tmux session '%s' not found. Create it with: tmux new-session -s %s", g.session, g.session)
		}
		return fmt.Errorf("failed to start tmux: %w", err)
	}

	if err := g.discoverWindows(); err != nil {
		g.process.Process.Kill()
		return fmt.Errorf("failed to discover windows: %w", err)
	}

	g.wg.Add(1)
	go g.reader(stdout)

	return nil
}

func (g *Gateway) discoverWindows() error {
	_, err := g.stdin.Write([]byte("list-windows -F '#{window_id} #{window_name} #{window_active}'\n"))
	if err != nil {
		return err
	}
	return nil
}

func (g *Gateway) reader(r io.Reader) {
	defer g.wg.Done()

	scanner := bufio.NewScanner(r)
	var inCommand bool
	var commandBuffer []string

	for scanner.Scan() {
		line := scanner.Text()

		event, err := ParseLine(line)
		if err != nil {
			continue
		}

		switch event.Type {
		case EventBegin:
			inCommand = true
			commandBuffer = nil
		case EventEnd, EventError:
			inCommand = false
			g.handleCommandResponse(commandBuffer)
			commandBuffer = nil
		default:
			if inCommand {
				commandBuffer = append(commandBuffer, line)
			} else {
				g.handleEvent(event)
			}
		}

		select {
		case g.events <- event:
		default:
		}
	}

	close(g.events)
	close(g.done)
}

func (g *Gateway) handleCommandResponse(lines []string) {
	for _, line := range lines {
		g.parseWindowLine(line)
	}
}

func (g *Gateway) parseWindowLine(line string) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return
	}

	windowID := parts[0]
	if len(windowID) == 0 || windowID[0] != '@' {
		return
	}

	name := parts[1]
	active := len(parts) > 2 && parts[2] == "1"

	g.mu.Lock()
	g.windows[windowID] = &Window{
		ID:     windowID,
		Name:   name,
		Active: active,
	}
	g.mu.Unlock()
}

func (g *Gateway) handleEvent(event Event) {
	switch event.Type {
	case EventWindowAdd:
		g.mu.Lock()
		g.windows[event.WindowID] = &Window{
			ID:   event.WindowID,
			Name: "",
		}
		g.mu.Unlock()
	case EventWindowClose:
		g.mu.Lock()
		delete(g.windows, event.WindowID)
		g.mu.Unlock()
	case EventWindowRenamed:
		g.mu.Lock()
		if w, ok := g.windows[event.WindowID]; ok {
			w.Name = event.Data
		}
		g.mu.Unlock()
	}
}

func (g *Gateway) Stop() error {
	if g.stdin != nil {
		g.stdin.Close()
	}
	if g.process != nil && g.process.Process != nil {
		g.process.Process.Kill()
	}
	g.wg.Wait()
	if g.process != nil {
		g.process.Wait()
	}
	return nil
}

func (g *Gateway) Events() <-chan Event {
	return g.events
}

func (g *Gateway) SendKeys(windowID string, keys string) error {
	if g.stdin == nil {
		return fmt.Errorf("gateway not started")
	}

	escaped := g.escapeKeys(keys)
	cmd := fmt.Sprintf("send-keys -t %s %s\n", windowID, escaped)
	_, err := g.stdin.Write([]byte(cmd))
	return err
}

func (g *Gateway) escapeKeys(keys string) string {
	if keys == "\n" || keys == "Enter" {
		return "Enter"
	}
	if keys == "\x03" {
		return "C-c"
	}

	result := strings.ReplaceAll(keys, "\n", " Enter ")
	result = strings.ReplaceAll(result, "\x03", " C-c ")
	result = strings.ReplaceAll(result, "\x1b", " Escape ")

	needsQuote := strings.ContainsAny(result, " '\"\t")
	if needsQuote {
		result = strings.ReplaceAll(result, "'", "'\\''")
		return "'" + result + "'"
	}

	return result
}

func (g *Gateway) ListWindows() []Window {
	g.mu.RLock()
	defer g.mu.RUnlock()

	windows := make([]Window, 0, len(g.windows))
	for _, w := range g.windows {
		windows = append(windows, *w)
	}
	return windows
}
