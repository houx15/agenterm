package orchestrator

import "testing"

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
