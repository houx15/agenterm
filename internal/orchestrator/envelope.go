package orchestrator

import (
	"encoding/json"
	"strings"
)

type AssistantConfirmation struct {
	Needed bool   `json:"needed"`
	Prompt string `json:"prompt,omitempty"`
}

type AssistantEnvelope struct {
	Discussion   string                `json:"discussion,omitempty"`
	Commands     []string              `json:"commands,omitempty"`
	StateUpdate  map[string]any        `json:"state_update,omitempty"`
	Confirmation AssistantConfirmation `json:"confirmation,omitempty"`
}

func ParseAssistantEnvelopeText(raw string) []AssistantEnvelope {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}
	chunks := extractJSONObjectChunks(text)
	if len(chunks) == 0 {
		return nil
	}
	result := make([]AssistantEnvelope, 0, len(chunks))
	for _, chunk := range chunks {
		var envelope AssistantEnvelope
		if err := json.Unmarshal([]byte(chunk), &envelope); err != nil {
			continue
		}
		result = append(result, envelope)
	}
	return result
}

func extractJSONObjectChunks(raw string) []string {
	chunks := make([]string, 0, 1)
	depth := 0
	inString := false
	escaped := false
	start := -1

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '{' {
			if depth == 0 {
				start = i
			}
			depth++
			continue
		}
		if ch == '}' {
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				chunks = append(chunks, raw[start:i+1])
				start = -1
			}
		}
	}
	return chunks
}
