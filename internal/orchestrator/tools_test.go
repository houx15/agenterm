package orchestrator

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func TestProgressiveDisclosureSkillTools(t *testing.T) {
	ts := NewToolset(nil)

	listRaw, err := ts.Execute(context.Background(), "list_skills", map[string]any{})
	if err != nil {
		t.Fatalf("list_skills failed: %v", err)
	}
	listMap, ok := listRaw.(map[string]any)
	if !ok {
		t.Fatalf("list_skills type=%T want map[string]any", listRaw)
	}
	skillsRaw, ok := listMap["skills"].([]map[string]string)
	if !ok {
		t.Fatalf("skills type=%T want []map[string]string", listMap["skills"])
	}
	if len(skillsRaw) == 0 {
		t.Fatalf("skills should be non-empty")
	}

	detailRaw, err := ts.Execute(context.Background(), "get_skill_details", map[string]any{"skill_id": "model-allocation"})
	if err != nil {
		t.Fatalf("get_skill_details failed: %v", err)
	}
	detail, ok := detailRaw.(map[string]any)
	if !ok {
		t.Fatalf("detail type=%T want map[string]any", detailRaw)
	}
	if detail["id"] != "model-allocation" {
		t.Fatalf("detail id=%v want model-allocation", detail["id"])
	}
	if _, ok := detail["details"].(string); !ok {
		t.Fatalf("details type=%T want string", detail["details"])
	}
	if _, ok := detail["description"].(string); !ok {
		t.Fatalf("description type=%T want string", detail["description"])
	}

	if _, err := ts.Execute(context.Background(), "get_skill_details", map[string]any{"skill_id": "unknown-skill"}); err == nil {
		t.Fatalf("expected unknown skill error")
	}
}

func TestInstallOnlineSkill(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	const skillContent = `---
name: sample-online-skill
description: Sample online skill.
---
# Sample Online Skill

Details for online-installed skill.
`
	originalClient := skillDownloadClient
	skillDownloadClient = &http.Client{
		Transport: toolsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(skillContent)),
				Header:     http.Header{"Content-Type": []string{"text/markdown"}},
			}, nil
		}),
	}
	t.Cleanup(func() { skillDownloadClient = originalClient })

	ts := NewToolset(nil)
	raw, err := ts.Execute(context.Background(), "install_online_skill", map[string]any{
		"url": "https://raw.githubusercontent.com/anthropics/skills/main/skills/sample-online-skill/SKILL.md",
	})
	if err != nil {
		t.Fatalf("install_online_skill failed: %v", err)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("result type=%T want map[string]any", raw)
	}
	if m["id"] != "sample-online-skill" {
		t.Fatalf("id=%v want sample-online-skill", m["id"])
	}
	if _, err := os.Stat(filepath.Join(tmp, "skills", "sample-online-skill", "SKILL.md")); err != nil {
		t.Fatalf("installed skill file missing: %v", err)
	}
}

type toolsRoundTripFunc func(req *http.Request) (*http.Response, error)

func (f toolsRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
