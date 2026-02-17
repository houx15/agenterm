package session

import "testing"

func TestAutoAcceptSequence(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		want   string
		accept bool
	}{
		{name: "empty", mode: "", want: "", accept: false},
		{name: "optional disabled", mode: "optional", want: "", accept: false},
		{name: "supported newline", mode: "supported", want: "\n", accept: true},
		{name: "shift tab", mode: "shift+tab", want: "\x1b[Z", accept: true},
		{name: "ctrl c", mode: "ctrl+c", want: "\x03", accept: true},
		{name: "escaped newline", mode: `\n`, want: "\n", accept: true},
		{name: "raw text fallback", mode: "foo", want: "foo", accept: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := autoAcceptSequence(tt.mode)
			if ok != tt.accept {
				t.Fatalf("autoAcceptSequence(%q) accepted=%v want %v", tt.mode, ok, tt.accept)
			}
			if got != tt.want {
				t.Fatalf("autoAcceptSequence(%q) sequence=%q want %q", tt.mode, got, tt.want)
			}
		})
	}
}
