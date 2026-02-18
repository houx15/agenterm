package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnforceCommandPolicyAllowsSafeCommand(t *testing.T) {
	root := t.TempDir()
	if err := enforceCommandPolicy("echo hello\n", root); err != nil {
		t.Fatalf("expected safe command to pass, got %v", err)
	}
}

func TestEnforceCommandPolicyBlocksTraversal(t *testing.T) {
	root := t.TempDir()
	err := enforceCommandPolicy("cat ../secret.txt\n", root)
	if err == nil {
		t.Fatalf("expected traversal command to be blocked")
	}
	if !IsCommandPolicyError(err) {
		t.Fatalf("expected command policy error, got %T", err)
	}
}

func TestEnforceCommandPolicyBlocksRmRfAbsolute(t *testing.T) {
	root := t.TempDir()
	err := enforceCommandPolicy("rm -rf /tmp/data\n", root)
	if err == nil {
		t.Fatalf("expected rm -rf absolute to be blocked")
	}
	if !IsCommandPolicyError(err) {
		t.Fatalf("expected command policy error, got %T", err)
	}
}

func TestEnforceCommandPolicyBlocksQuotedRmRfAbsolute(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"rm -rf '/tmp/data'\n",
		"rm -rf \"/tmp/data\"\n",
	}
	for _, tc := range cases {
		err := enforceCommandPolicy(tc, root)
		if err == nil {
			t.Fatalf("expected quoted rm -rf absolute to be blocked: %q", tc)
		}
		if !IsCommandPolicyError(err) {
			t.Fatalf("expected command policy error, got %T for %q", err, tc)
		}
	}
}

func TestEnforceCommandPolicyBlocksWrappedRmRfAbsolute(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"sudo rm -rf /tmp/data\n",
		"env rm -rf /tmp/data\n",
		"command rm -rf /tmp/data\n",
	}
	for _, tc := range cases {
		err := enforceCommandPolicy(tc, root)
		if err == nil {
			t.Fatalf("expected wrapped rm command to be blocked: %q", tc)
		}
		if !IsCommandPolicyError(err) {
			t.Fatalf("expected command policy error, got %T for %q", err, tc)
		}
	}
}

func TestEnforceCommandPolicyBlocksPathOutsideWorkdir(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	err := enforceCommandPolicy("cat "+outside+"\n", root)
	if err == nil {
		t.Fatalf("expected outside path command to be blocked")
	}
	if !IsCommandPolicyError(err) {
		t.Fatalf("expected command policy error, got %T", err)
	}
}

func TestEnforceCommandPolicyBlocksEnvExpandedPath(t *testing.T) {
	root := t.TempDir()
	err := enforceCommandPolicy("cat $HOME/.ssh/id_rsa\n", root)
	if err == nil {
		t.Fatalf("expected env expanded path to be blocked")
	}
	if !IsCommandPolicyError(err) {
		t.Fatalf("expected command policy error, got %T", err)
	}
}

func TestEnforceCommandPolicyFailsClosedWhenWorkDirUnknownAndPathPresent(t *testing.T) {
	err := enforceCommandPolicy("cat ./README.md\n", "")
	if err == nil {
		t.Fatalf("expected path command to be blocked when workdir is unknown")
	}
	if !IsCommandPolicyError(err) {
		t.Fatalf("expected command policy error, got %T", err)
	}
}

func TestEnforceCommandPolicyAllowsNoPathCommandWhenWorkDirUnknown(t *testing.T) {
	if err := enforceCommandPolicy("pwd\n", ""); err != nil {
		t.Fatalf("expected no-path command to pass, got %v", err)
	}
}

func TestAuditCommandPolicyViolationWritesLog(t *testing.T) {
	root := t.TempDir()
	policyErr := &CommandPolicyError{
		Rule:    "test_rule",
		Detail:  "test detail",
		Command: "bad",
	}
	auditCommandPolicyViolation(root, "session-1", "bad", policyErr)
	auditPath := filepath.Join(root, ".orchestra", "command-policy-audit.log")
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected non-empty audit log")
	}
}
