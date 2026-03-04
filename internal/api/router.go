package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/hub"
	"github.com/user/agenterm/internal/registry"
	"github.com/user/agenterm/internal/session"
)

type handler struct {
	projectRepo        *db.ProjectRepo
	taskRepo           *db.TaskRepo
	worktreeRepo       *db.WorktreeRepo
	sessionRepo        *db.SessionRepo
	sessionCommandRepo *db.SessionCommandRepo
	knowledgeRepo      *db.ProjectKnowledgeRepo
	reviewRepo         *db.ReviewRepo
	runRepo            *db.RunRepo
	demandPoolRepo     *db.DemandPoolRepo
	requirementRepo      *db.RequirementRepo
	planningSessionRepo    *db.PlanningSessionRepo
	permissionTemplateRepo *db.PermissionTemplateRepo
	registry               *registry.Registry
	lifecycle          *session.Manager
	hub                *hub.Hub

	outputMu    sync.Mutex
	outputState map[string]*windowOutputState
}

func NewRouter(conn *sql.DB, lifecycle *session.Manager, hubInst *hub.Hub, token string, agentRegistry *registry.Registry) http.Handler {
	handler := &handler{
		projectRepo:        db.NewProjectRepo(conn),
		taskRepo:           db.NewTaskRepo(conn),
		worktreeRepo:       db.NewWorktreeRepo(conn),
		sessionRepo:        db.NewSessionRepo(conn),
		sessionCommandRepo: db.NewSessionCommandRepo(conn),
		knowledgeRepo:      db.NewProjectKnowledgeRepo(conn),
		reviewRepo:         db.NewReviewRepo(conn),
		runRepo:            db.NewRunRepo(conn),
		demandPoolRepo:     db.NewDemandPoolRepo(conn),
		requirementRepo:      db.NewRequirementRepo(conn),
		planningSessionRepo:    db.NewPlanningSessionRepo(conn),
		permissionTemplateRepo: db.NewPermissionTemplateRepo(conn),
		registry:               agentRegistry,
		lifecycle:          lifecycle,
		hub:                hubInst,
		outputState:        make(map[string]*windowOutputState),
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
	mux.HandleFunc("POST /api/worktrees/{id}/merge", handler.mergeWorktree)
	mux.HandleFunc("POST /api/worktrees/{id}/resolve-conflict", handler.resolveWorktreeConflict)
	mux.HandleFunc("DELETE /api/worktrees/{id}", handler.deleteWorktree)

	mux.HandleFunc("POST /api/tasks/{id}/sessions", handler.createSession)
	mux.HandleFunc("GET /api/sessions", handler.listSessions)
	mux.HandleFunc("GET /api/sessions/{id}", handler.getSession)
	mux.HandleFunc("POST /api/sessions/{id}/send", handler.sendSessionCommand)
	mux.HandleFunc("POST /api/sessions/{id}/send-key", handler.sendSessionKey)
	mux.HandleFunc("POST /api/sessions/{id}/commands", handler.enqueueSessionCommand)
	mux.HandleFunc("GET /api/sessions/{id}/commands", handler.listSessionCommands)
	mux.HandleFunc("GET /api/sessions/{id}/commands/{command_id}", handler.getSessionCommand)
	mux.HandleFunc("GET /api/sessions/{id}/output", handler.getSessionOutput)
	mux.HandleFunc("GET /api/sessions/{id}/idle", handler.getSessionIdle)
	mux.HandleFunc("GET /api/sessions/{id}/ready", handler.getSessionReady)
	mux.HandleFunc("GET /api/sessions/{id}/close-check", handler.getSessionCloseCheck)
	mux.HandleFunc("PATCH /api/sessions/{id}/takeover", handler.patchSessionTakeover)
	mux.HandleFunc("DELETE /api/sessions/{id}", handler.deleteSession)

	mux.HandleFunc("GET /api/agents", handler.listAgents)
	mux.HandleFunc("GET /api/agents/status", handler.listAgentStatuses)
	mux.HandleFunc("GET /api/agents/{id}", handler.getAgent)
	mux.HandleFunc("POST /api/agents", handler.createAgent)
	mux.HandleFunc("PUT /api/agents/{id}", handler.updateAgent)
	mux.HandleFunc("DELETE /api/agents/{id}", handler.deleteAgent)
	mux.HandleFunc("GET /api/fs/directories", handler.listDirectories)

	mux.HandleFunc("GET /api/projects/{id}/runs/current", handler.getCurrentProjectRun)
	mux.HandleFunc("POST /api/projects/{id}/runs/{run_id}/transition", handler.transitionProjectRun)
	mux.HandleFunc("GET /api/projects/{id}/knowledge", handler.listProjectKnowledge)
	mux.HandleFunc("POST /api/projects/{id}/knowledge", handler.createProjectKnowledge)
	mux.HandleFunc("GET /api/tasks/{id}/review-cycles", handler.listTaskReviewCycles)
	mux.HandleFunc("GET /api/tasks/{id}/review-loop/status", handler.getTaskReviewLoopStatus)
	mux.HandleFunc("POST /api/tasks/{id}/review-cycles", handler.createTaskReviewCycle)
	mux.HandleFunc("PATCH /api/review-cycles/{id}", handler.updateReviewCycle)
	mux.HandleFunc("GET /api/review-cycles/{id}/issues", handler.listReviewCycleIssues)
	mux.HandleFunc("POST /api/review-cycles/{id}/issues", handler.createReviewCycleIssue)
	mux.HandleFunc("PATCH /api/review-issues/{id}", handler.updateReviewIssue)

	mux.HandleFunc("GET /api/projects/{id}/demand-pool", handler.listDemandPoolItems)
	mux.HandleFunc("POST /api/projects/{id}/demand-pool", handler.createDemandPoolItem)
	mux.HandleFunc("POST /api/projects/{id}/demand-pool/reprioritize", handler.reprioritizeDemandPool)
	mux.HandleFunc("GET /api/demand-pool/{id}", handler.getDemandPoolItem)
	mux.HandleFunc("PATCH /api/demand-pool/{id}", handler.updateDemandPoolItem)
	mux.HandleFunc("DELETE /api/demand-pool/{id}", handler.deleteDemandPoolItem)
	mux.HandleFunc("POST /api/demand-pool/{id}/promote", handler.promoteDemandPoolItem)

	mux.HandleFunc("POST /api/projects/{id}/requirements", handler.createRequirement)
	mux.HandleFunc("GET /api/projects/{id}/requirements", handler.listRequirements)
	mux.HandleFunc("GET /api/requirements/{id}", handler.getRequirement)
	mux.HandleFunc("PATCH /api/requirements/{id}", handler.updateRequirement)
	mux.HandleFunc("DELETE /api/requirements/{id}", handler.deleteRequirement)
	mux.HandleFunc("POST /api/projects/{id}/requirements/reorder", handler.reorderRequirements)

	mux.HandleFunc("POST /api/requirements/{id}/planning", handler.createPlanningSession)
	mux.HandleFunc("GET /api/requirements/{id}/planning", handler.getPlanningSession)
	mux.HandleFunc("PATCH /api/planning-sessions/{id}", handler.updatePlanningSession)
	mux.HandleFunc("POST /api/planning-sessions/{id}/blueprint", handler.saveBlueprint)

	mux.HandleFunc("POST /api/requirements/{id}/launch", handler.launchExecution)
	mux.HandleFunc("POST /api/requirements/{id}/transition", handler.transitionStage)

	mux.HandleFunc("GET /api/permission-templates", handler.listPermissionTemplates)
	mux.HandleFunc("GET /api/permission-templates/{agent_type}", handler.listPermissionTemplatesByAgent)
	mux.HandleFunc("POST /api/permission-templates", handler.createPermissionTemplate)
	mux.HandleFunc("PUT /api/permission-templates/{id}", handler.updatePermissionTemplate)
	mux.HandleFunc("DELETE /api/permission-templates/{id}", handler.deletePermissionTemplate)

	mux.HandleFunc("GET /api/settings", handler.getSettings)
	mux.HandleFunc("PUT /api/settings", handler.updateSettings)

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
