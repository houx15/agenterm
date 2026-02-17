package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/user/agenterm/internal/playbook"
)

func TestPlaybookStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "invalid playbook",
			err:  fmt.Errorf("wrapped: %w", playbook.ErrInvalidPlaybook),
			want: http.StatusBadRequest,
		},
		{
			name: "not found",
			err:  fmt.Errorf("wrapped: %w", playbook.ErrPlaybookNotFound),
			want: http.StatusNotFound,
		},
		{
			name: "storage",
			err:  fmt.Errorf("wrapped: %w", playbook.ErrPlaybookStorage),
			want: http.StatusInternalServerError,
		},
		{
			name: "unknown",
			err:  errors.New("boom"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := playbookStatusCode(tc.err)
			if got != tc.want {
				t.Fatalf("playbookStatusCode() = %d want %d", got, tc.want)
			}
		})
	}
}
