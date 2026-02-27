package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/orchestrator"
	"github.com/user/agenterm/internal/playbook"
	"github.com/user/agenterm/internal/registry"
)

type flowHarness struct {
	db               *db.DB
	projectRepo      *db.ProjectRepo
	taskRepo         *db.TaskRepo
	worktreeRepo     *db.WorktreeRepo
	sessionRepo      *db.SessionRepo
	historyRepo      *db.OrchestratorHistoryRepo
	playbookRegistry *playbook.Registry
	agentRegistry    *registry.Registry
	orch             *orchestrator.Orchestrator
	apiServer        *httptest.Server
	llmServer        *httptest.Server
	gw               *fakeGateway
}

type llmRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content []struct {
			Type      string         `json:"type"`
			ID        string         `json:"id,omitempty"`
			Name      string         `json:"name,omitempty"`
			Input     map[string]any `json:"input,omitempty"`
			ToolUseID string         `json:"tool_use_id,omitempty"`
			Content   any            `json:"content,omitempty"`
			Text      string         `json:"text,omitempty"`
		} `json:"content"`
	} `json:"messages"`
}

func TestOrchestratorInterfaceFlow_Slice1_PlanPreflight(t *testing.T) {
	var reqLog []llmRequest
	var plannerSessionID string

	h := newFlowHarness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		req := decodeLLMRequest(t, r)
		reqLog = append(reqLog, req)
		step := len(reqLog)
		switch step {
		case 1:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("plan-create-session", "create_session", map[string]any{
						"task_id":    "TASK_PLAN",
						"agent_type": "planner-agent",
						"role":       "requirements-analyst",
						"inputs": map[string]any{
							"goal": "Create implementation plan and specs",
						},
					}),
				},
			})
		case 2:
			result := latestToolResult(req, "plan-create-session")
			plannerSessionID = stringValue(result, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("plan-send", "send_command", map[string]any{
						"session_id": plannerSessionID,
						"text":       "Analyze repository and draft staged plan with spec list.\n",
					}),
				},
			})
		case 3:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("plan-read", "read_session_output", map[string]any{
						"session_id": plannerSessionID,
						"lines":      40,
					}),
				},
			})
		case 4:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("plan-spec", "write_task_spec", map[string]any{
						"project_id":    "PROJECT_PLAN",
						"relative_path": "docs/specs/plan-task.md",
						"content":       "# Plan Spec\n\n- Stage 1: Plan\n- Stage 2: Build\n- Stage 3: Test\n",
					}),
				},
			})
		default:
			writeJSON(w, map[string]any{
				"content": []any{
					textBlock(`{"discussion":"Planning complete. Spec written under docs/specs/plan-task.md.","commands":[],"confirmation":{"needed":false,"prompt":""}}`),
				},
			})
		}
	})

	repoPath := filepath.Join(t.TempDir(), "project-plan")
	project := &db.Project{
		ID:       "PROJECT_PLAN",
		Name:     "PlanProject",
		RepoPath: repoPath,
		Status:   "planning",
		Playbook: "test-flow",
	}
	if err := h.projectRepo.Create(context.Background(), project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{
		ID:          "TASK_PLAN",
		ProjectID:   project.ID,
		Title:       "Plan task",
		Description: "Plan and spec generation",
		Status:      "pending",
	}
	if err := h.taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	oldCapture := capturePaneFn
	capturePaneFn = func(windowID string, lines int) ([]string, error) {
		return []string{
			"PLAN: split into 3 stages",
			"SPEC: docs/specs/plan-task.md",
		}, nil
	}
	defer func() { capturePaneFn = oldCapture }()

	events := runOrchestratorChat(t, h.orch, project.ID, "Confirm.")
	assertNoStreamError(t, events)
	assertToolCalled(t, events, "create_session")
	assertToolCalled(t, events, "send_command")
	assertToolCalled(t, events, "read_session_output")
	assertToolCalled(t, events, "write_task_spec")

	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		t.Fatalf("expected repo to be git-initialized: %v", err)
	}
	specPath := filepath.Join(repoPath, "docs/specs/plan-task.md")
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("expected spec to be written at %s: %v", specPath, err)
	}
	if len(h.gw.sentRaw) == 0 {
		t.Fatalf("expected send_command to route to gateway")
	}
	if !containsAny(h.gw.sentRaw, "Analyze repository and draft staged plan") {
		t.Fatalf("expected planner prompt to be sent, got=%v", h.gw.sentRaw)
	}

	status := fetchAgentStatus(t, h.apiServer.URL, "test-token")
	if status.TotalBusy == 0 {
		t.Fatalf("expected at least one busy agent after plan run, got %+v", status)
	}
}

func TestOrchestratorInterfaceFlow_Slice2_BuildFlow(t *testing.T) {
	var reqLog []llmRequest
	var worktreeID string
	var coderSessionID string
	var reviewerSessionID string

	h := newFlowHarness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		req := decodeLLMRequest(t, r)
		reqLog = append(reqLog, req)
		step := len(reqLog)
		switch step {
		case 1:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("build-worktree", "create_worktree", map[string]any{
						"project_id":  "PROJECT_BUILD",
						"task_id":     "TASK_BUILD",
						"branch_name": "feature/build-flow",
						"path":        ".worktrees/build-flow",
					}),
				},
			})
		case 2:
			wt := latestToolResult(req, "build-worktree")
			worktreeID = stringValue(wt, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("build-coder-session", "create_session", map[string]any{
						"task_id":    "TASK_BUILD",
						"agent_type": "coder-agent",
						"role":       "coder",
						"inputs": map[string]any{
							"worktree_id": worktreeID,
							"spec_path":   "docs/specs/build-task.md",
						},
					}),
				},
			})
		case 3:
			sess := latestToolResult(req, "build-coder-session")
			coderSessionID = stringValue(sess, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("build-coder-send", "send_command", map[string]any{
						"session_id": coderSessionID,
						"text":       "Implement according to docs/specs/build-task.md and summarize changed files.\n",
					}),
				},
			})
		case 4:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("build-reviewer-session", "create_session", map[string]any{
						"task_id":    "TASK_BUILD",
						"agent_type": "reviewer-agent",
						"role":       "reviewer",
					}),
				},
			})
		case 5:
			sess := latestToolResult(req, "build-reviewer-session")
			reviewerSessionID = stringValue(sess, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("build-reviewer-send", "send_command", map[string]any{
						"session_id": reviewerSessionID,
						"text":       "Review branch feature/build-flow against docs/specs/build-task.md and list defects.\n",
					}),
				},
			})
		case 6:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("build-merge", "merge_worktree", map[string]any{
						"worktree_id": worktreeID,
					}),
				},
			})
		default:
			writeJSON(w, map[string]any{
				"content": []any{
					textBlock(`{"discussion":"Build stage complete. Worktree merged successfully.","commands":[],"confirmation":{"needed":false,"prompt":""}}`),
				},
			})
		}
	})

	repoPath := filepath.Join(t.TempDir(), "project-build")
	initGitRepoWithInitialCommit(t, repoPath)

	project := &db.Project{
		ID:       "PROJECT_BUILD",
		Name:     "BuildProject",
		RepoPath: repoPath,
		Status:   "building",
		Playbook: "test-flow",
	}
	if err := h.projectRepo.Create(context.Background(), project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{
		ID:          "TASK_BUILD",
		ProjectID:   project.ID,
		Title:       "Build task",
		Description: "Implement planned feature",
		Status:      "running",
		SpecPath:    "docs/specs/build-task.md",
	}
	if err := h.taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	events := runOrchestratorChat(t, h.orch, project.ID, "Confirm.")
	assertNoStreamError(t, events)
	assertToolCalled(t, events, "create_worktree")
	assertToolCalled(t, events, "create_session")
	assertToolCalled(t, events, "send_command")
	assertToolCalled(t, events, "merge_worktree")

	worktree, err := h.worktreeRepo.Get(context.Background(), worktreeID)
	if err != nil || worktree == nil {
		t.Fatalf("expected created worktree %q, err=%v", worktreeID, err)
	}
	if worktree.Status != "merged" {
		t.Fatalf("worktree status=%q want merged", worktree.Status)
	}
	if _, err := os.Stat(worktree.Path); err != nil {
		t.Fatalf("expected worktree path to exist: %v", err)
	}
	if !containsAny(h.gw.sentRaw, "Implement according to docs/specs/build-task.md") {
		t.Fatalf("expected coder prompt send, got=%v", h.gw.sentRaw)
	}
	if !containsAny(h.gw.sentRaw, "Review branch feature/build-flow") {
		t.Fatalf("expected reviewer prompt send, got=%v", h.gw.sentRaw)
	}

	status := fetchAgentStatus(t, h.apiServer.URL, "test-token")
	if status.TotalBusy < 2 {
		t.Fatalf("expected at least two busy agents in build flow, got %+v", status)
	}
}

func TestOrchestratorInterfaceFlow_Slice3_TestFlowAndHumanGuidance(t *testing.T) {
	var reqLog []llmRequest
	var testerSessionID string

	h := newFlowHarness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		req := decodeLLMRequest(t, r)
		reqLog = append(reqLog, req)
		step := len(reqLog)
		switch step {
		case 1:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("test-session", "create_session", map[string]any{
						"task_id":    "TASK_TEST",
						"agent_type": "tester-agent",
						"role":       "qa-tester",
					}),
				},
			})
		case 2:
			sess := latestToolResult(req, "test-session")
			testerSessionID = stringValue(sess, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("test-send", "send_command", map[string]any{
						"session_id": testerSessionID,
						"text":       "Create and execute test plan from docs/specs/test-task.md, then report manual checks.\n",
					}),
				},
			})
		case 3:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("test-read", "read_session_output", map[string]any{
						"session_id": testerSessionID,
						"lines":      60,
					}),
				},
			})
		default:
			writeJSON(w, map[string]any{
				"content": []any{
					textBlock(`{"discussion":"Automated checks passed. Human follow-up: run browser smoke test on Safari and validate keyboard navigation.","commands":[],"confirmation":{"needed":false,"prompt":""}}`),
				},
			})
		}
	})

	repoPath := filepath.Join(t.TempDir(), "project-test")
	initGitRepoWithInitialCommit(t, repoPath)

	project := &db.Project{
		ID:       "PROJECT_TEST",
		Name:     "TestProject",
		RepoPath: repoPath,
		Status:   "testing",
		Playbook: "test-flow",
	}
	if err := h.projectRepo.Create(context.Background(), project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{
		ID:          "TASK_TEST",
		ProjectID:   project.ID,
		Title:       "Test task",
		Description: "Validate delivered feature",
		Status:      "done",
		SpecPath:    "docs/specs/test-task.md",
	}
	if err := h.taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	specPath := filepath.Join(repoPath, "docs/specs/test-task.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	oldCapture := capturePaneFn
	capturePaneFn = func(windowID string, lines int) ([]string, error) {
		return []string{
			"PASS: unit tests",
			"TODO: manual browser keyboard verification",
		}, nil
	}
	defer func() { capturePaneFn = oldCapture }()

	events := runOrchestratorChat(t, h.orch, project.ID, "Confirm.")
	assertNoStreamError(t, events)
	assertToolCalled(t, events, "create_session")
	assertToolCalled(t, events, "send_command")
	assertToolCalled(t, events, "read_session_output")

	if !containsAny(h.gw.sentRaw, "Create and execute test plan") {
		t.Fatalf("expected tester prompt send, got=%v", h.gw.sentRaw)
	}
	allTokenText := collectTokenText(events)
	if !strings.Contains(strings.ToLower(allTokenText), "human follow-up") {
		t.Fatalf("expected human follow-up guidance in final text, got=%q", allTokenText)
	}

	status := fetchAgentStatus(t, h.apiServer.URL, "test-token")
	if status.TotalBusy == 0 {
		t.Fatalf("expected busy tester assignment, got %+v", status)
	}
}

func TestOrchestratorInterfaceFlow_Slice4_GuardrailRegression(t *testing.T) {
	h := newFlowHarness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		writeJSON(w, map[string]any{
			"content": []any{
				toolUseBlock("forbidden", "create_worktree", map[string]any{
					"project_id":  "PROJECT_GUARD",
					"task_id":     "TASK_GUARD",
					"branch_name": "feature/should-not-run",
				}),
			},
		})
	})

	repoPath := filepath.Join(t.TempDir(), "project-guard")
	initGitRepoWithInitialCommit(t, repoPath)
	project := &db.Project{
		ID:       "PROJECT_GUARD",
		Name:     "GuardProject",
		RepoPath: repoPath,
		Status:   "testing",
		Playbook: "test-flow",
	}
	if err := h.projectRepo.Create(context.Background(), project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{
		ID:          "TASK_GUARD",
		ProjectID:   project.ID,
		Title:       "Guard task",
		Description: "Ensure stage gate blocks build tool in test stage",
		Status:      "done",
	}
	if err := h.taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	events := runOrchestratorChat(t, h.orch, project.ID, "Confirm.")
	var foundStageGate bool
	for _, evt := range events {
		if evt.Type != "tool_result" {
			continue
		}
		body, _ := json.Marshal(evt.Result)
		if bytes.Contains(body, []byte("stage_tool_not_allowed")) {
			foundStageGate = true
			break
		}
	}
	if !foundStageGate {
		t.Fatalf("expected stage_tool_not_allowed guardrail result, events=%+v", events)
	}
}

func TestOrchestratorInterfaceFlow_Slice4_EndToEndLifecycleSingleProject(t *testing.T) {
	var reqLog []llmRequest
	var runID string
	var plannerSessionID string
	var coderSessionID string
	var reviewerSessionID string
	var testerSessionID string
	var worktreeID string
	var reviewCycleID string
	var reviewIssueID string

	h := newFlowHarness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		req := decodeLLMRequest(t, r)
		reqLog = append(reqLog, req)
		step := len(reqLog)

		switch step {
		case 1:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-run", "get_current_run", map[string]any{
						"project_id": "PROJECT_E2E",
					}),
				},
			})
		case 2:
			runResp := latestToolResult(req, "e2e-run")
			runID = nestedString(runResp, "run", "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-plan-session", "create_session", map[string]any{
						"task_id":    "TASK_E2E",
						"agent_type": "planner-agent",
						"role":       "requirements-analyst",
						"inputs": map[string]any{
							"goal": "Plan staged implementation and generate specs",
						},
					}),
				},
			})
		case 3:
			created := latestToolResult(req, "e2e-plan-session")
			plannerSessionID = stringValue(created, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-plan-send", "send_command", map[string]any{
						"session_id": plannerSessionID,
						"text":       "Analyze repository and write staged implementation plan with worktree split.\n",
					}),
				},
			})
		case 4:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-plan-read", "read_session_output", map[string]any{
						"session_id": plannerSessionID,
						"lines":      80,
					}),
				},
			})
		case 5:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-plan-spec", "write_task_spec", map[string]any{
						"project_id":    "PROJECT_E2E",
						"relative_path": "docs/specs/e2e-feature.md",
						"content":       "# E2E Feature Spec\n\n- stage: plan\n- stage: build\n- stage: test\n",
					}),
				},
			})
		case 6:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-to-build", "transition_run_stage", map[string]any{
						"project_id": "PROJECT_E2E",
						"run_id":     runID,
						"to_stage":   "build",
						"status":     "active",
						"evidence": map[string]any{
							"reason": "plan approved by user",
						},
					}),
				},
			})
		case 7:
			writeJSON(w, map[string]any{
				"content": []any{
					textBlock(`{"discussion":"Plan stage complete and transitioned to build.","commands":[],"confirmation":{"needed":false,"prompt":""}}`),
				},
			})
		case 8:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-build-worktree", "create_worktree", map[string]any{
						"project_id":  "PROJECT_E2E",
						"task_id":     "TASK_E2E",
						"branch_name": "feature/e2e-flow",
						"path":        ".worktrees/e2e-flow",
					}),
				},
			})
		case 9:
			wtResp := latestToolResult(req, "e2e-build-worktree")
			worktreeID = stringValue(wtResp, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-coder-session", "create_session", map[string]any{
						"task_id":    "TASK_E2E",
						"agent_type": "coder-agent",
						"role":       "coder",
						"inputs": map[string]any{
							"worktree_id": worktreeID,
							"spec_path":   "docs/specs/e2e-feature.md",
						},
					}),
				},
			})
		case 10:
			created := latestToolResult(req, "e2e-coder-session")
			coderSessionID = stringValue(created, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-coder-send", "send_command", map[string]any{
						"session_id": coderSessionID,
						"text":       "Implement feature from docs/specs/e2e-feature.md and summarize changes.\n",
					}),
				},
			})
		case 11:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-review-cycle", "create_review_cycle", map[string]any{
						"task_id":     "TASK_E2E",
						"commit_hash": "abc1234",
					}),
				},
			})
		case 12:
			cycle := latestToolResult(req, "e2e-review-cycle")
			reviewCycleID = stringValue(cycle, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-review-issue", "create_review_issue", map[string]any{
						"cycle_id": reviewCycleID,
						"summary":  "Input validation missing for keyboard edge case",
						"severity": "high",
					}),
				},
			})
		case 13:
			issue := latestToolResult(req, "e2e-review-issue")
			reviewIssueID = stringValue(issue, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-loop-status-needs-fix", "get_review_loop_status", map[string]any{
						"task_id": "TASK_E2E",
					}),
				},
			})
		case 14:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-reviewer-session", "create_session", map[string]any{
						"task_id":    "TASK_E2E",
						"agent_type": "reviewer-agent",
						"role":       "reviewer",
					}),
				},
			})
		case 15:
			created := latestToolResult(req, "e2e-reviewer-session")
			reviewerSessionID = stringValue(created, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-reviewer-send", "send_command", map[string]any{
						"session_id": reviewerSessionID,
						"text":       "Review feature/e2e-flow against docs/specs/e2e-feature.md and verify fix quality.\n",
					}),
				},
			})
		case 16:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-review-issue-resolve", "update_review_issue", map[string]any{
						"issue_id":    reviewIssueID,
						"status":      "resolved",
						"resolution":  "Added explicit keyboard input validation checks.",
						"severity":    "medium",
						"summary":     "Input validation updated",
					}),
				},
			})
		case 17:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-loop-status-passed", "get_review_loop_status", map[string]any{
						"task_id": "TASK_E2E",
					}),
				},
			})
		case 18:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-merge", "merge_worktree", map[string]any{
						"worktree_id": worktreeID,
					}),
				},
			})
		case 19:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-to-test", "transition_run_stage", map[string]any{
						"project_id": "PROJECT_E2E",
						"run_id":     runID,
						"to_stage":   "test",
						"status":     "active",
						"evidence": map[string]any{
							"reason": "build merged and review loop passed",
						},
					}),
				},
			})
		case 20:
			writeJSON(w, map[string]any{
				"content": []any{
					textBlock(`{"discussion":"Build stage complete and transitioned to test.","commands":[],"confirmation":{"needed":false,"prompt":""}}`),
				},
			})
		case 21:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-test-session", "create_session", map[string]any{
						"task_id":    "TASK_E2E",
						"agent_type": "tester-agent",
						"role":       "qa-tester",
					}),
				},
			})
		case 22:
			created := latestToolResult(req, "e2e-test-session")
			testerSessionID = stringValue(created, "id")
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-test-send", "send_command", map[string]any{
						"session_id": testerSessionID,
						"text":       "Create test plan from docs/specs/e2e-feature.md, execute automated checks, report manual follow-ups.\n",
					}),
				},
			})
		case 23:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-test-read", "read_session_output", map[string]any{
						"session_id": testerSessionID,
						"lines":      80,
					}),
				},
			})
		case 24:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-summary", "create_project_knowledge", map[string]any{
						"project_id": "PROJECT_E2E",
						"kind":       "final_summary",
						"title":      "E2E lifecycle summary",
						"content":    "Plan/build/test completed with review loop passed and manual QA follow-ups listed.",
					}),
				},
			})
		case 25:
			writeJSON(w, map[string]any{
				"content": []any{
					toolUseBlock("e2e-test-complete", "transition_run_stage", map[string]any{
						"project_id": "PROJECT_E2E",
						"run_id":     runID,
						"to_stage":   "test",
						"status":     "completed",
						"evidence": map[string]any{
							"reason": "test execution complete",
						},
					}),
				},
			})
		case 26:
			writeJSON(w, map[string]any{
				"content": []any{
					textBlock(`{"discussion":"Lifecycle complete: plan, build, and test finished with review loop pass and final summary saved.","commands":[],"confirmation":{"needed":false,"prompt":""}}`),
				},
			})
		default:
			writeJSON(w, map[string]any{
				"content": []any{
					textBlock(`{"discussion":"No further actions.","commands":[],"confirmation":{"needed":false,"prompt":""}}`),
				},
			})
		}
	})

	repoPath := filepath.Join(t.TempDir(), "project-e2e")
	initGitRepoWithInitialCommit(t, repoPath)
	project := &db.Project{
		ID:       "PROJECT_E2E",
		Name:     "E2EProject",
		RepoPath: repoPath,
		Status:   "planning",
		Playbook: "test-flow",
	}
	if err := h.projectRepo.Create(context.Background(), project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{
		ID:          "TASK_E2E",
		ProjectID:   project.ID,
		Title:       "Implement e2e lifecycle feature",
		Description: "Drive plan-build-test flow end-to-end",
		Status:      "pending",
	}
	if err := h.taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	oldCapture := capturePaneFn
	capturePaneFn = func(windowID string, lines int) ([]string, error) {
		return []string{
			"READY: worker prompt accepted",
			"DONE: execution checkpoint",
		}, nil
	}
	defer func() { capturePaneFn = oldCapture }()

	events := runOrchestratorChat(t, h.orch, project.ID, "Confirm.")
	assertNoStreamError(t, events)
	assertNoToolResultErrors(t, events)
	assertToolCalled(t, events, "get_current_run")
	assertToolCalled(t, events, "create_session")
	assertToolCalled(t, events, "write_task_spec")
	assertToolCalled(t, events, "transition_run_stage")

	events = runOrchestratorChat(t, h.orch, project.ID, "Confirm.")
	assertNoStreamError(t, events)
	assertNoToolResultErrors(t, events)
	assertToolCalled(t, events, "create_worktree")
	assertToolCalled(t, events, "create_review_cycle")
	assertToolCalled(t, events, "create_review_issue")
	assertToolCalled(t, events, "update_review_issue")
	assertToolCalled(t, events, "merge_worktree")

	events = runOrchestratorChat(t, h.orch, project.ID, "Confirm.")
	assertNoStreamError(t, events)
	assertNoToolResultErrors(t, events)
	assertToolCalled(t, events, "create_project_knowledge")

	specPath := filepath.Join(repoPath, "docs/specs/e2e-feature.md")
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("expected e2e spec at %s: %v", specPath, err)
	}

	wt, err := h.worktreeRepo.Get(context.Background(), worktreeID)
	if err != nil || wt == nil {
		t.Fatalf("expected worktree %q to exist, err=%v", worktreeID, err)
	}
	if strings.TrimSpace(strings.ToLower(wt.Status)) != "merged" {
		t.Fatalf("worktree status=%q want merged", wt.Status)
	}

	loop := fetchTaskReviewLoopStatus(t, h.apiServer.URL, "test-token", task.ID)
	if !boolValue(loop, "passed") {
		t.Fatalf("expected review loop passed, got=%v", loop)
	}
	if boolValue(loop, "needs_fix") {
		t.Fatalf("expected review loop not needing fix, got=%v", loop)
	}

	knowledge := fetchProjectKnowledgeEntries(t, h.apiServer.URL, "test-token", project.ID)
	if !containsKnowledgeKind(knowledge, "final_summary") {
		t.Fatalf("expected final_summary project knowledge entry, got=%v", knowledge)
	}

	runRepo := db.NewRunRepo(h.db.SQL())
	persistedRun, err := runRepo.Get(context.Background(), runID)
	if err != nil || persistedRun == nil {
		t.Fatalf("load persisted run %q failed: %v", runID, err)
	}
	if got := strings.ToLower(strings.TrimSpace(persistedRun.CurrentStage)); got != "test" {
		t.Fatalf("persisted run current_stage=%q want test", got)
	}
	if got := strings.ToLower(strings.TrimSpace(persistedRun.Status)); got != "completed" {
		t.Fatalf("persisted run status=%q want completed", got)
	}

	if !containsAny(h.gw.sentRaw, "Analyze repository and write staged implementation plan") {
		t.Fatalf("expected planner prompt send, got=%v", h.gw.sentRaw)
	}
	if !containsAny(h.gw.sentRaw, "Implement feature from docs/specs/e2e-feature.md") {
		t.Fatalf("expected coder prompt send, got=%v", h.gw.sentRaw)
	}
	if !containsAny(h.gw.sentRaw, "Review feature/e2e-flow against docs/specs/e2e-feature.md") {
		t.Fatalf("expected reviewer prompt send, got=%v", h.gw.sentRaw)
	}
	if !containsAny(h.gw.sentRaw, "Create test plan from docs/specs/e2e-feature.md") {
		t.Fatalf("expected tester prompt send, got=%v", h.gw.sentRaw)
	}
}

func newFlowHarness(t *testing.T, llmHandler http.HandlerFunc) *flowHarness {
	t.Helper()
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "flow-test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	agentRegistry, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	playbookRegistry, err := playbook.NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("new playbook registry: %v", err)
	}
	saveFlowAgents(t, agentRegistry)
	saveFlowPlaybook(t, playbookRegistry)

	gw := &fakeGateway{}
	router := NewRouter(database.SQL(), gw, nil, nil, nil, nil, nil, "test-token", "configured-session", agentRegistry, playbookRegistry)
	apiServer := httptest.NewServer(router)
	t.Cleanup(apiServer.Close)

	llmServer := httptest.NewServer(llmHandler)
	t.Cleanup(llmServer.Close)

	h := &flowHarness{
		db:               database,
		projectRepo:      db.NewProjectRepo(database.SQL()),
		taskRepo:         db.NewTaskRepo(database.SQL()),
		worktreeRepo:     db.NewWorktreeRepo(database.SQL()),
		sessionRepo:      db.NewSessionRepo(database.SQL()),
		historyRepo:      db.NewOrchestratorHistoryRepo(database.SQL()),
		playbookRegistry: playbookRegistry,
		agentRegistry:    agentRegistry,
		apiServer:        apiServer,
		llmServer:        llmServer,
		gw:               gw,
	}

	h.orch = orchestrator.New(orchestrator.Options{
		APIKey:                  "test-key",
		Model:                   "test-model",
		AnthropicBaseURL:        llmServer.URL + "/v1/messages",
		APIToolBaseURL:          apiServer.URL,
		APIToken:                "test-token",
		ProjectRepo:             h.projectRepo,
		TaskRepo:                h.taskRepo,
		WorktreeRepo:            h.worktreeRepo,
		SessionRepo:             h.sessionRepo,
		HistoryRepo:             h.historyRepo,
		RunRepo:                 db.NewRunRepo(database.SQL()),
		ProjectOrchestratorRepo: db.NewProjectOrchestratorRepo(database.SQL()),
		WorkflowRepo:            db.NewWorkflowRepo(database.SQL()),
		KnowledgeRepo:           db.NewProjectKnowledgeRepo(database.SQL()),
		RoleBindingRepo:         db.NewRoleBindingRepo(database.SQL()),
		RoleLoopAttemptRepo:     db.NewRoleLoopAttemptRepo(database.SQL()),
		Registry:                h.agentRegistry,
		PlaybookRegistry:        h.playbookRegistry,
		MaxToolRounds:           16,
		GlobalMaxParallel:       16,
	})
	return h
}

func saveFlowAgents(t *testing.T, reg *registry.Registry) {
	t.Helper()
	agents := []*registry.AgentConfig{
		{ID: "planner-agent", Name: "Planner Agent", Model: "planner-model", Command: "planner", MaxParallelAgents: 1},
		{ID: "coder-agent", Name: "Coder Agent", Model: "coder-model", Command: "coder", MaxParallelAgents: 4},
		{ID: "reviewer-agent", Name: "Reviewer Agent", Model: "reviewer-model", Command: "reviewer", MaxParallelAgents: 2},
		{ID: "tester-agent", Name: "Tester Agent", Model: "tester-model", Command: "tester", MaxParallelAgents: 2},
	}
	for _, agent := range agents {
		if err := reg.Save(agent); err != nil {
			t.Fatalf("save agent %s: %v", agent.ID, err)
		}
	}
}

func saveFlowPlaybook(t *testing.T, reg *playbook.Registry) {
	t.Helper()
	pb := &playbook.Playbook{
		ID:          "test-flow",
		Name:        "Test Flow",
		Description: "Playbook for orchestrator interface flow tests",
		Workflow: playbook.Workflow{
			Plan: playbook.Stage{
				Enabled: true,
				Roles: []playbook.StageRole{
					{
						Name:             "requirements-analyst",
						Mode:             "planner",
						Responsibilities: "Plan and spec authoring",
						AllowedAgents:    []string{"planner-agent"},
						InputsRequired:   []string{"goal"},
						ActionsAllowed: []string{
							"get_project_status",
							"create_session",
							"send_command",
							"read_session_output",
							"write_task_spec",
						},
					},
				},
			},
			Build: playbook.Stage{
				Enabled: true,
				Roles: []playbook.StageRole{
					{
						Name:             "coder",
						Mode:             "worker",
						Responsibilities: "Implement feature",
						AllowedAgents:    []string{"coder-agent"},
						ActionsAllowed: []string{
							"get_project_status",
							"create_worktree",
							"create_session",
							"send_command",
							"read_session_output",
							"merge_worktree",
						},
					},
					{
						Name:             "reviewer",
						Mode:             "reviewer",
						Responsibilities: "Review changes",
						AllowedAgents:    []string{"reviewer-agent"},
						ActionsAllowed: []string{
							"get_project_status",
							"create_session",
							"send_command",
							"read_session_output",
						},
					},
				},
			},
			Test: playbook.Stage{
				Enabled: true,
				Roles: []playbook.StageRole{
					{
						Name:             "qa-tester",
						Mode:             "tester",
						Responsibilities: "Validate quality and produce human follow-up checklist",
						AllowedAgents:    []string{"tester-agent"},
						ActionsAllowed: []string{
							"get_project_status",
							"create_session",
							"send_command",
							"read_session_output",
						},
					},
				},
			},
		},
	}
	if err := reg.Save(pb); err != nil {
		t.Fatalf("save flow playbook: %v", err)
	}
}

func runOrchestratorChat(t *testing.T, o *orchestrator.Orchestrator, projectID, message string) []orchestrator.StreamEvent {
	t.Helper()
	stream, err := o.Chat(context.Background(), projectID, message)
	if err != nil {
		t.Fatalf("chat error: %v", err)
	}
	events := make([]orchestrator.StreamEvent, 0, 32)
	for evt := range stream {
		events = append(events, evt)
	}
	return events
}

func assertNoStreamError(t *testing.T, events []orchestrator.StreamEvent) {
	t.Helper()
	for _, evt := range events {
		if evt.Type == "error" {
			t.Fatalf("unexpected stream error: %s", evt.Error)
		}
	}
}

func assertToolCalled(t *testing.T, events []orchestrator.StreamEvent, toolName string) {
	t.Helper()
	for _, evt := range events {
		if evt.Type == "tool_call" && strings.TrimSpace(evt.Name) == strings.TrimSpace(toolName) {
			return
		}
	}
	t.Fatalf("expected tool_call for %q, events=%+v", toolName, events)
}

func assertNoToolResultErrors(t *testing.T, events []orchestrator.StreamEvent) {
	t.Helper()
	for _, evt := range events {
		if evt.Type != "tool_result" {
			continue
		}
		payload, ok := evt.Result.(map[string]any)
		if !ok || payload == nil {
			continue
		}
		if errText := strings.TrimSpace(stringValue(payload, "error")); errText != "" {
			t.Fatalf("unexpected tool_result error for %q: %v", evt.Name, payload)
		}
	}
}

func collectTokenText(events []orchestrator.StreamEvent) string {
	var b strings.Builder
	for _, evt := range events {
		if evt.Type == "token" {
			b.WriteString(evt.Text)
		}
	}
	return b.String()
}

func latestToolResult(req llmRequest, toolUseID string) map[string]any {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		for _, block := range req.Messages[i].Content {
			if block.Type != "tool_result" || strings.TrimSpace(block.ToolUseID) != strings.TrimSpace(toolUseID) {
				continue
			}
			switch typed := block.Content.(type) {
			case string:
				out := map[string]any{}
				if err := json.Unmarshal([]byte(typed), &out); err == nil {
					return out
				}
			case map[string]any:
				return typed
			}
		}
	}
	return map[string]any{}
}

func stringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

func nestedString(m map[string]any, parent string, key string) string {
	if m == nil {
		return ""
	}
	raw, ok := m[strings.TrimSpace(parent)]
	if !ok {
		return ""
	}
	child, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(child, key)
}

func toolUseBlock(id, name string, input map[string]any) map[string]any {
	return map[string]any{
		"type":  "tool_use",
		"id":    id,
		"name":  name,
		"input": input,
	}
}

func textBlock(text string) map[string]any {
	return map[string]any{
		"type": "text",
		"text": text,
	}
}

func decodeLLMRequest(t *testing.T, r *http.Request) llmRequest {
	t.Helper()
	defer r.Body.Close()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read llm request: %v", err)
	}
	var req llmRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("decode llm request: %v body=%s", err, string(raw))
	}
	return req
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

type agentStatusSnapshot struct {
	TotalBusy int `json:"total_busy"`
}

func fetchAgentStatus(t *testing.T, baseURL string, token string) agentStatusSnapshot {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/agents/status", nil)
	if err != nil {
		t.Fatalf("new status request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status endpoint code=%d body=%s", resp.StatusCode, string(body))
	}
	var out agentStatusSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	return out
}

func fetchTaskReviewLoopStatus(t *testing.T, baseURL string, token string, taskID string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/tasks/"+taskID+"/review-loop/status", nil)
	if err != nil {
		t.Fatalf("new review-loop request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("review-loop request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("review-loop endpoint code=%d body=%s", resp.StatusCode, string(body))
	}
	out := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode review-loop response: %v", err)
	}
	return out
}

func fetchProjectKnowledgeEntries(t *testing.T, baseURL string, token string, projectID string) []map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/projects/"+projectID+"/knowledge", nil)
	if err != nil {
		t.Fatalf("new knowledge request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("knowledge request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("knowledge endpoint code=%d body=%s", resp.StatusCode, string(body))
	}
	var out []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode knowledge response: %v", err)
	}
	return out
}

func fetchCurrentRun(t *testing.T, baseURL string, token string, projectID string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/projects/"+projectID+"/runs/current", nil)
	if err != nil {
		t.Fatalf("new current-run request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("current-run request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("current-run endpoint code=%d body=%s", resp.StatusCode, string(body))
	}
	out := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode current-run response: %v", err)
	}
	return out
}

func boolValue(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	raw, ok := m[key]
	if !ok {
		return false
	}
	v, ok := raw.(bool)
	return ok && v
}

func containsKnowledgeKind(entries []map[string]any, kind string) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	for _, entry := range entries {
		if strings.ToLower(strings.TrimSpace(stringValue(entry, "kind"))) == kind {
			return true
		}
	}
	return false
}

func containsAny(values []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func initGitRepoWithInitialCommit(t *testing.T, repoPath string) {
	t.Helper()
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	runGitCmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if out, err := cmd.Output(); err != nil {
			t.Fatalf("git %s failed: %v stderr=%s stdout=%s", strings.Join(args, " "), err, stderr.String(), string(out))
		}
	}

	runGitCmd("init")
	runGitCmd("config", "user.email", "test@agenterm.local")
	runGitCmd("config", "user.name", "agenterm-test")

	readme := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readme, []byte("# Test Repo\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	runGitCmd("add", "README.md")
	runGitCmd("commit", "-m", "init")

	branch := detectBranchName(t, repoPath)
	if branch != "main" {
		runGitCmd("checkout", "-B", "main")
	}
}

func detectBranchName(t *testing.T, repoPath string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("detect branch failed: %v stderr=%s", err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}

func (h *flowHarness) String() string {
	if h == nil {
		return "nil harness"
	}
	return fmt.Sprintf("flowHarness(api=%s llm=%s)", h.apiServer.URL, h.llmServer.URL)
}
