package orchestrator

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSummarizeProjectStatusIncludesPhaseQueueAndBlockers(t *testing.T) {
	report := summarizeProjectStatus(map[string]any{
		"project": map[string]any{"id": "p1"},
		"tasks": []any{
			map[string]any{"id": "t1", "status": "pending"},
			map[string]any{"id": "t2", "status": "blocked"},
		},
		"sessions": []any{
			map[string]any{"id": "s1", "status": "failed"},
		},
		"worktrees": []any{},
	})

	if report["phase"] != "blocked" {
		t.Fatalf("phase=%v want blocked", report["phase"])
	}
	if report["queue_depth"] != 1 {
		t.Fatalf("queue_depth=%v want 1", report["queue_depth"])
	}
	blockers, ok := report["blockers"].([]any)
	if !ok {
		t.Fatalf("blockers type=%T want []any", report["blockers"])
	}
	if len(blockers) == 0 {
		t.Fatalf("blockers should be non-empty")
	}
}

func TestMergeToolsCallExpectedEndpoints(t *testing.T) {
	var calls []string
	client := &RESTToolClient{
		BaseURL: "http://example.test",
		HTTPClient: &http.Client{
			Transport: toolsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				bodyText := ""
				if req.Body != nil {
					b, _ := io.ReadAll(req.Body)
					bodyText = strings.TrimSpace(string(b))
				}
				calls = append(calls, req.Method+" "+req.URL.Path+" "+bodyText)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}),
		},
	}
	ts := NewToolset(client)

	if _, err := ts.Execute(context.Background(), "merge_worktree", map[string]any{
		"worktree_id":   "wt-1",
		"target_branch": "main",
	}); err != nil {
		t.Fatalf("merge_worktree failed: %v", err)
	}
	if _, err := ts.Execute(context.Background(), "resolve_merge_conflict", map[string]any{
		"worktree_id": "wt-1",
		"session_id":  "s-1",
		"message":     "fix please",
	}); err != nil {
		t.Fatalf("resolve_merge_conflict failed: %v", err)
	}
	if _, err := ts.Execute(context.Background(), "can_close_session", map[string]any{
		"session_id": "s-1",
	}); err != nil {
		t.Fatalf("can_close_session failed: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("calls=%d want 3", len(calls))
	}
	if !strings.HasPrefix(calls[0], "POST /api/worktrees/wt-1/merge") {
		t.Fatalf("unexpected first call: %q", calls[0])
	}
	if !strings.HasPrefix(calls[1], "POST /api/worktrees/wt-1/resolve-conflict") {
		t.Fatalf("unexpected second call: %q", calls[1])
	}
	if !strings.HasPrefix(calls[2], "GET /api/sessions/s-1/close-check") {
		t.Fatalf("unexpected third call: %q", calls[2])
	}
}

type toolsRoundTripFunc func(req *http.Request) (*http.Response, error)

func (f toolsRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
