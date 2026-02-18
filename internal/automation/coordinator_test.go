package automation

import (
	"testing"
	"time"
)

func TestParseReviewDecisionApproved(t *testing.T) {
	dec := parseReviewDecision("VERDICT: APPROVED\nLooks good")
	if !dec.approved {
		t.Fatalf("expected approved decision")
	}
}

func TestParseReviewDecisionDoesNotApproveNegation(t *testing.T) {
	dec := parseReviewDecision("This is not approved yet; please fix tests")
	if dec.approved {
		t.Fatalf("did not expect approved")
	}
}

func TestParseReviewDecisionFeedback(t *testing.T) {
	dec := parseReviewDecision("Please add tests for edge cases")
	if dec.approved {
		t.Fatalf("did not expect approved")
	}
	if dec.feedback == "" {
		t.Fatalf("expected feedback")
	}
}

func TestJoinOutput(t *testing.T) {
	text := joinOutput([]OutputEntry{
		{Text: "", Timestamp: time.Now().UTC()},
		{Text: "first", Timestamp: time.Now().UTC()},
		{Text: " second ", Timestamp: time.Now().UTC()},
	})
	if text != "first\nsecond" {
		t.Fatalf("joinOutput=%q want %q", text, "first\\nsecond")
	}
}
