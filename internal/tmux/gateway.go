package tmux

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

var windowIDRe = regexp.MustCompile(`^@\d+$`)
var paneIDRe = regexp.MustCompile(`^%\d+$`)

type Gateway struct {
	session      string
	process      *exec.Cmd
	stdin        io.WriteCloser
	events       chan Event
	done         chan struct{}
	windows      map[string]*Window
	paneToWindow map[string]string
	mu           sync.RWMutex
	wg           sync.WaitGroup
}

func New(session string) *Gateway {
	return &Gateway{
		session:      session,
		events:       make(chan Event, 1000),
		done:         make(chan struct{}),
		windows:      make(map[string]*Window),
		paneToWindow: make(map[string]string),
	}
}

func (g *Gateway) Start(ctx context.Context) error {
	if err := g.checkSessionExists(); err != nil {
		return err
	}

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

func (g *Gateway) checkSessionExists() error {
	cmd := exec.Command("tmux", "has-session", "-t", g.session)
	err := cmd.Run()
	if err != nil {
		if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			return fmt.Errorf("tmux binary not found. Please install tmux")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return fmt.Errorf("tmux session '%s' not found. Create it with: tmux new-session -s %s", g.session, g.session)
			}
		}
		return fmt.Errorf("failed to check tmux session: %w", err)
	}
	return nil
}

func (g *Gateway) discoverWindows() error {
	_, err := g.stdin.Write([]byte(fmt.Sprintf("list-windows -t %s -F '#{window_id}\t#{window_name}\t#{window_active}'\n", g.session)))
	if err != nil {
		return err
	}
	_, err = g.stdin.Write([]byte(fmt.Sprintf("list-panes -a -t %s -F '#{pane_id}\t#{window_id}'\n", g.session)))
	if err != nil {
		return err
	}
	return nil
}

func (g *Gateway) reader(r io.Reader) {
	defer g.wg.Done()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var inCommand bool
	var commandBuffer []string

	for scanner.Scan() {
		line := scanner.Text()

		if inCommand {
			if strings.HasPrefix(line, "%end ") || strings.HasPrefix(line, "%error ") {
				event, _ := ParseLine(line)
				inCommand = false
				g.handleCommandResponse(commandBuffer)
				commandBuffer = nil

				select {
				case g.events <- event:
				default:
				}
			} else {
				commandBuffer = append(commandBuffer, line)
			}
			continue
		}

		event, err := ParseLine(line)
		if err != nil {
			continue
		}

		if event.Type == EventBegin {
			inCommand = true
			commandBuffer = nil
			select {
			case g.events <- event:
			default:
			}
			continue
		}

		if event.Type == EventOutput && event.WindowID == "" && event.PaneID != "" {
			g.mu.RLock()
			if windowID, ok := g.paneToWindow[event.PaneID]; ok {
				event.WindowID = windowID
			}
			g.mu.RUnlock()
		}

		g.handleEvent(event)

		select {
		case g.events <- event:
		default:
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("tmux reader error", "error", err)
	}

	close(g.events)
	close(g.done)
}

func (g *Gateway) handleCommandResponse(lines []string) {
	for _, line := range lines {
		g.parseWindowLine(line)
		g.parsePaneLine(line)
	}
}

func (g *Gateway) parseWindowLine(line string) {
	parts := strings.Split(line, "\t")
	if len(parts) < 2 {
		return
	}

	windowID := parts[0]
	if !windowIDRe.MatchString(windowID) {
		return
	}

	name := parts[1]
	active := len(parts) > 2 && strings.TrimSpace(parts[2]) == "1"

	g.mu.Lock()
	g.windows[windowID] = &Window{
		ID:     windowID,
		Name:   name,
		Active: active,
	}
	g.mu.Unlock()
}

func (g *Gateway) parsePaneLine(line string) {
	parts := strings.Split(line, "\t")
	if len(parts) < 2 {
		return
	}

	paneID := parts[0]
	windowID := parts[1]

	if !paneIDRe.MatchString(paneID) || !windowIDRe.MatchString(windowID) {
		return
	}

	g.mu.Lock()
	g.paneToWindow[paneID] = windowID
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

	if !windowIDRe.MatchString(windowID) {
		return fmt.Errorf("invalid window ID format: %s", windowID)
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
	if keys == "\x1b" {
		return "Escape"
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

func (g *Gateway) NewWindow(name string) error {
	if g.stdin == nil {
		return fmt.Errorf("gateway not started")
	}

	if name != "" {
		_, err := g.stdin.Write([]byte(fmt.Sprintf("new-window -n %s\n", name)))
		return err
	}
	_, err := g.stdin.Write([]byte("new-window\n"))
	return err
}
