package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/hub"
	"github.com/user/agenterm/internal/tmux"
)

type gateway interface {
	NewWindow(name, defaultDir string) error
	ListWindows() []tmux.Window
	SendKeys(windowID string, keys string) error
	SendRaw(windowID string, keys string) error
}

type handler struct {
	projectRepo  *db.ProjectRepo
	taskRepo     *db.TaskRepo
	worktreeRepo *db.WorktreeRepo
	sessionRepo  *db.SessionRepo
	agentRepo    *db.AgentConfigRepo
	gw           gateway
	hub          *hub.Hub
	tmuxSession  string

	outputMu    sync.Mutex
	outputState map[string]*windowOutputState
}

func NewRouter(conn *sql.DB, gw gateway, hubInst *hub.Hub, token string, tmuxSession string) http.Handler {
	handler := &handler{
		projectRepo:  db.NewProjectRepo(conn),
		taskRepo:     db.NewTaskRepo(conn),
		worktreeRepo: db.NewWorktreeRepo(conn),
		sessionRepo:  db.NewSessionRepo(conn),
		agentRepo:    db.NewAgentConfigRepo(conn),
		gw:           gw,
		hub:          hubInst,
		tmuxSession:  tmuxSession,
		outputState:  make(map[string]*windowOutputState),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/projects", handler.createProject)
	mux.HandleFunc("GET /api/projects", handler.listProjects)
	mux.HandleFunc("GET /api/projects/{id}", handler.getProject)
	mux.HandleFunc("PATCH /api/projects/{id}", handler.updateProject)
	mux.HandleFunc("DELETE /api/projects/{id}", handler.deleteProject)

	mux.HandleFunc("POST /api/projects/{id}/tasks", handler.createTask)
	mux.HandleFunc("GET /api/projects/{id}/tasks", handler.listTasks)
	mux.HandleFunc("GET /api/tasks/{id}", handler.getTask)
	mux.HandleFunc("PATCH /api/tasks/{id}", handler.updateTask)

	mux.HandleFunc("POST /api/projects/{id}/worktrees", handler.createWorktree)
	mux.HandleFunc("GET /api/worktrees/{id}/git-status", handler.getWorktreeGitStatus)
	mux.HandleFunc("GET /api/worktrees/{id}/git-log", handler.getWorktreeGitLog)
	mux.HandleFunc("DELETE /api/worktrees/{id}", handler.deleteWorktree)

	mux.HandleFunc("POST /api/tasks/{id}/sessions", handler.createSession)
	mux.HandleFunc("GET /api/sessions", handler.listSessions)
	mux.HandleFunc("GET /api/sessions/{id}", handler.getSession)
	mux.HandleFunc("POST /api/sessions/{id}/send", handler.sendSessionCommand)
	mux.HandleFunc("GET /api/sessions/{id}/output", handler.getSessionOutput)
	mux.HandleFunc("GET /api/sessions/{id}/idle", handler.getSessionIdle)
	mux.HandleFunc("PATCH /api/sessions/{id}/takeover", handler.patchSessionTakeover)

	mux.HandleFunc("GET /api/agents", handler.listAgents)
	mux.HandleFunc("GET /api/agents/{id}", handler.getAgent)

	wrapped := authMiddleware(token)(jsonMiddleware(corsMiddleware(mux)))
	return wrapped
}

func authMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				if strings.TrimSpace(authHeader[7:]) == token {
					next.ServeHTTP(w, r)
					return
				}
			}

			if r.URL.Query().Get("token") == token {
				next.ServeHTTP(w, r)
				return
			}

			jsonError(w, http.StatusUnauthorized, "unauthorized")
		})
	}
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func defaultAgentsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "agenterm", "agents")
}
