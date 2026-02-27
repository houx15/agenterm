package pty

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	creackpty "github.com/creack/pty"
)

// Session wraps a child process running inside a PTY.
type Session struct {
	id        string
	name      string
	createdAt time.Time

	cmd  *exec.Cmd
	ptmx *os.File

	events chan Event

	cols uint16
	rows uint16

	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
}

// newSession spawns a command inside a new PTY and returns the Session.
// The PTY is created with a default size of 120 columns x 30 rows.
func newSession(id, name string, argv []string, workDir string, env []string) (*Session, error) {
	if len(argv) == 0 {
		return nil, errors.New("pty: argv must not be empty")
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = workDir
	if len(env) > 0 {
		cmd.Env = env
	}

	defaultCols := uint16(120)
	defaultRows := uint16(30)

	ptmx, err := creackpty.StartWithSize(cmd, &creackpty.Winsize{
		Cols: defaultCols,
		Rows: defaultRows,
	})
	if err != nil {
		return nil, err
	}

	s := &Session{
		id:        id,
		name:      name,
		createdAt: time.Now(),
		cmd:       cmd,
		ptmx:      ptmx,
		events:    make(chan Event, 1024),
		cols:      defaultCols,
		rows:      defaultRows,
	}

	go s.readPump()
	go s.waitExit()

	return s, nil
}

// readPump reads data from the PTY fd and sends EventOutput events.
// It runs until the PTY is closed or any read error occurs.
func (s *Session) readPump() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			s.events <- Event{
				Type: EventOutput,
				ID:   s.id,
				Data: string(buf[:n]),
			}
		}
		if err != nil {
			break
		}
	}
}

// waitExit waits for the child process to exit, then sends an EventClosed
// event and closes the events channel.
func (s *Session) waitExit() {
	_ = s.cmd.Wait()

	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	s.events <- Event{
		Type: EventClosed,
		ID:   s.id,
	}
	close(s.events)
}

// ID returns the session identifier.
func (s *Session) ID() string { return s.id }

// Name returns the human-readable session name.
func (s *Session) Name() string { return s.name }

// Events returns the read-only channel of session events.
func (s *Session) Events() <-chan Event { return s.events }

// Write sends data to the PTY (and therefore to the child process's stdin).
func (s *Session) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, errors.New("pty: session is closed")
	}
	return s.ptmx.Write(data)
}

// Resize changes the PTY window size.
func (s *Session) Resize(cols, rows uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("pty: session is closed")
	}

	if err := creackpty.Setsize(s.ptmx, &creackpty.Winsize{
		Cols: cols,
		Rows: rows,
	}); err != nil {
		return err
	}

	s.cols = cols
	s.rows = rows
	return nil
}

// Close terminates the child process (SIGTERM) and closes the PTY fd.
// It is safe to call Close multiple times.
func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()

		// Send SIGTERM to the child process if it is still running.
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Signal(syscall.SIGTERM)
		}

		err = s.ptmx.Close()
	})
	return err
}
