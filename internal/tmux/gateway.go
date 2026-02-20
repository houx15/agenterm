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
	"time"
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
	closeOnce    sync.Once
	closeErr     error
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

	g.wg.Add(1)
	go g.reader(stdout)

	if err := g.discoverWindows(); err != nil {
		g.process.Process.Kill()
		return fmt.Errorf("failed to discover windows: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

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
				if event, err := ParseLine(line); err == nil {
					if event.Type == EventOutput && event.WindowID == "" && event.PaneID != "" {
						event.WindowID = g.resolveWindowIDForPane(event.PaneID)
					}
					g.handleEvent(event)
					select {
					case g.events <- event:
					default:
					}
				}
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
			event.WindowID = g.resolveWindowIDForPane(event.PaneID)
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

func (g *Gateway) resolveWindowIDForPane(paneID string) string {
	if paneID == "" {
		return ""
	}

	g.mu.RLock()
	if windowID, ok := g.paneToWindow[paneID]; ok && windowID != "" {
		g.mu.RUnlock()
		return windowID
	}
	g.mu.RUnlock()

	// Pane/window mappings can change when tmux is controlled outside this process.
	// Do a best-effort sync and retry lookup before dropping terminal output.
	if err := g.syncStateFromTmux(); err != nil {
		return ""
	}

	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.paneToWindow[paneID]
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
		delete(g.paneToWindow, event.PaneID)
		g.mu.Unlock()
	case EventWindowRenamed:
		g.mu.Lock()
		if w, ok := g.windows[event.WindowID]; ok {
			w.Name = event.Data
		}
		g.mu.Unlock()
	case EventLayoutChange:
	}
}

func (g *Gateway) Stop() error {
	g.closeOnce.Do(func() {
		if g.stdin != nil {
			if err := g.stdin.Close(); err != nil {
				g.closeErr = err
			}
			g.stdin = nil
		}

		if g.process != nil && g.process.Process != nil {
			_ = g.process.Process.Kill()
		}

		g.wg.Wait()

		if g.process != nil {
			_ = g.process.Wait()
		}
	})
	return g.closeErr
}

func (g *Gateway) Close() error {
	return g.Stop()
}

func (g *Gateway) SessionName() string {
	return g.session
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

	commands := g.buildSendKeysCommands(windowID, keys)
	for _, cmd := range commands {
		if _, err := g.stdin.Write([]byte(cmd)); err != nil {
			return err
		}
	}
	return nil
}

func (g *Gateway) SendRaw(windowID string, keys string) error {
	if g.stdin == nil {
		return fmt.Errorf("gateway not started")
	}
	if !windowIDRe.MatchString(windowID) {
		return fmt.Errorf("invalid window ID format: %s", windowID)
	}

	data := []byte(keys)
	if len(data) == 0 {
		return nil
	}

	const chunkSize = 64
	pending := make([]byte, 0, chunkSize)
	flushPending := func() error {
		if len(pending) == 0 {
			return nil
		}
		var b strings.Builder
		b.WriteString("send-keys -t ")
		b.WriteString(windowID)
		for _, ch := range pending {
			b.WriteString(fmt.Sprintf(" -H %02x", ch))
		}
		b.WriteByte('\n')
		if _, err := g.stdin.Write([]byte(b.String())); err != nil {
			return err
		}
		pending = pending[:0]
		return nil
	}

	for _, ch := range data {
		if ch == '\n' || ch == '\r' {
			if err := flushPending(); err != nil {
				return err
			}
			if _, err := g.stdin.Write([]byte(fmt.Sprintf("send-keys -t %s C-m\n", windowID))); err != nil {
				return err
			}
			continue
		}
		pending = append(pending, ch)
		if len(pending) >= chunkSize {
			if err := flushPending(); err != nil {
				return err
			}
		}
	}
	if err := flushPending(); err != nil {
		return err
	}

	return nil
}

func (g *Gateway) ResizeWindow(windowID string, cols int, rows int) error {
	if g.stdin == nil {
		return fmt.Errorf("gateway not started")
	}
	if !windowIDRe.MatchString(windowID) {
		return fmt.Errorf("invalid window ID format: %s", windowID)
	}
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("invalid terminal size: %dx%d", cols, rows)
	}

	_, err := g.stdin.Write([]byte(fmt.Sprintf("refresh-client -C %dx%d\n", cols, rows)))
	return err
}

func (g *Gateway) buildSendKeysCommands(windowID string, keys string) []string {
	switch keys {
	case "\n", "Enter":
		return []string{fmt.Sprintf("send-keys -t %s Enter\n", windowID)}
	case "C-m":
		return []string{fmt.Sprintf("send-keys -t %s C-m\n", windowID)}
	case "\x03", "C-c":
		return []string{fmt.Sprintf("send-keys -t %s C-c\n", windowID)}
	case "\x1b", "Escape":
		return []string{fmt.Sprintf("send-keys -t %s Escape\n", windowID)}
	case "\t", "Tab":
		return []string{fmt.Sprintf("send-keys -t %s Tab\n", windowID)}
	}

	var cmds []string
	var literal strings.Builder
	flushLiteral := func() {
		if literal.Len() == 0 {
			return
		}
		cmds = append(cmds, fmt.Sprintf("send-keys -t %s -l -- %s\n", windowID, tmuxQuote(literal.String())))
		literal.Reset()
	}

	for i := 0; i < len(keys); i++ {
		switch keys[i] {
		case '\n':
			flushLiteral()
			cmds = append(cmds, fmt.Sprintf("send-keys -t %s Enter\n", windowID))
		case '\x03':
			flushLiteral()
			cmds = append(cmds, fmt.Sprintf("send-keys -t %s C-c\n", windowID))
		case '\x1b':
			flushLiteral()
			cmds = append(cmds, fmt.Sprintf("send-keys -t %s Escape\n", windowID))
		case '\t':
			flushLiteral()
			cmds = append(cmds, fmt.Sprintf("send-keys -t %s Tab\n", windowID))
		default:
			literal.WriteByte(keys[i])
		}
	}
	flushLiteral()
	return cmds
}

func tmuxQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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

func (g *Gateway) NewWindow(name string, defaultDir string) error {
	if g.stdin == nil {
		return fmt.Errorf("gateway not started")
	}

	if name != "" {
		_, err := g.stdin.Write([]byte(fmt.Sprintf("new-window -n %s\n", name)))
		if err != nil {
			return err
		}
	} else {
		_, err := g.stdin.Write([]byte("new-window\n"))
		if err != nil {
			return err
		}
	}

	if defaultDir != "" {
		_, err := g.stdin.Write([]byte(fmt.Sprintf("send-keys 'cd %s' Enter\n", defaultDir)))
		if err != nil {
			return err
		}
	}

	for i := 0; i < 5; i++ {
		if err := g.syncStateFromTmux(); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	return nil
}

func (g *Gateway) KillWindow(windowID string) error {
	if g.stdin == nil {
		return fmt.Errorf("gateway not started")
	}

	if !windowIDRe.MatchString(windowID) {
		return fmt.Errorf("invalid window ID format: %s", windowID)
	}

	if _, err := g.stdin.Write([]byte(fmt.Sprintf("kill-window -t %s\n", windowID))); err != nil {
		return err
	}

	for i := 0; i < 5; i++ {
		if err := g.syncStateFromTmux(); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (g *Gateway) syncStateFromTmux() error {
	windowsCmd := exec.Command("tmux", "list-windows", "-t", g.session, "-F", "#{window_id}\t#{window_name}\t#{window_active}")
	windowsOut, err := windowsCmd.Output()
	if err != nil {
		return fmt.Errorf("list-windows failed: %w", err)
	}

	panesCmd := exec.Command("tmux", "list-panes", "-a", "-t", g.session, "-F", "#{pane_id}\t#{window_id}")
	panesOut, err := panesCmd.Output()
	if err != nil {
		return fmt.Errorf("list-panes failed: %w", err)
	}

	windows := parseWindowsOutput(string(windowsOut))
	paneToWindow := parsePanesOutput(string(panesOut))

	g.mu.Lock()
	g.windows = windows
	g.paneToWindow = paneToWindow
	g.mu.Unlock()

	return nil
}

func parseWindowsOutput(output string) map[string]*Window {
	result := make(map[string]*Window)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		windowID := parts[0]
		if !windowIDRe.MatchString(windowID) {
			continue
		}
		name := parts[1]
		active := len(parts) > 2 && strings.TrimSpace(parts[2]) == "1"
		result[windowID] = &Window{
			ID:     windowID,
			Name:   name,
			Active: active,
		}
	}
	return result
}

func parsePanesOutput(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		paneID := parts[0]
		windowID := parts[1]
		if !paneIDRe.MatchString(paneID) || !windowIDRe.MatchString(windowID) {
			continue
		}
		result[paneID] = windowID
	}
	return result
}
