package orchestrator

import "testing"

func TestParseAssistantEnvelopeText(t *testing.T) {
	t.Parallel()

	raw := `{"discussion":"d1","commands":["c1"],"state_update":{"stage":"plan"},"confirmation":{"needed":false}}{"discussion":"d2","commands":["c2"],"confirmation":{"needed":true,"prompt":"go?"}}`
	items := ParseAssistantEnvelopeText(raw)
	if len(items) != 2 {
		t.Fatalf("expected 2 envelopes, got %d", len(items))
	}
	if items[0].Discussion != "d1" {
		t.Fatalf("unexpected first discussion: %q", items[0].Discussion)
	}
	if len(items[0].Commands) != 1 || items[0].Commands[0] != "c1" {
		t.Fatalf("unexpected first commands: %#v", items[0].Commands)
	}
	if got := items[0].StateUpdate["stage"]; got != "plan" {
		t.Fatalf("unexpected state_update.stage: %#v", got)
	}
	if !items[1].Confirmation.Needed {
		t.Fatalf("expected second confirmation needed")
	}
	if items[1].Confirmation.Prompt != "go?" {
		t.Fatalf("unexpected second confirmation prompt: %q", items[1].Confirmation.Prompt)
	}
}

func TestParseAssistantEnvelopeText_IgnoreMalformedChunk(t *testing.T) {
	t.Parallel()

	raw := `{"discussion":"ok"}{"discussion":bad}{"discussion":"ok2"}`
	items := ParseAssistantEnvelopeText(raw)
	if len(items) != 2 {
		t.Fatalf("expected 2 valid envelopes, got %d", len(items))
	}
}
