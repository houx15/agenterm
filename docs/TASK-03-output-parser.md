# Task: output-parser

## Context
Agenterm transforms raw terminal output into chat-friendly messages. The Output Parser is the "intelligence" layer that accumulates terminal output per window, segments it into discrete messages, and classifies each message (normal, prompt, error, code).

**Tech stack:** Go 1.22+, stdlib only.

## Objective
Implement a `parser.Parser` that consumes raw terminal output events and produces classified, segmented `Message` objects suitable for display in a chat UI.

## Dependencies
- Depends on: feature/02-tmux-gateway (needs `tmux.Event` types)
- Branch: feature/03-output-parser
- Base: main

## Scope

### Files to Create
- `internal/parser/types.go` — Message types and classifications
- `internal/parser/parser.go` — Accumulator + classifier logic
- `internal/parser/ansi.go` — ANSI escape code stripping
- `internal/parser/patterns.go` — Regex patterns for prompt/error/code detection

### Files to Modify
- None (standalone package)

### Files NOT to Touch
- `internal/tmux/` — consume its types but don't modify

## Implementation Spec

### Step 1: Define Types (`internal/parser/types.go`)

```go
package parser

import "time"

type MessageClass string

const (
    ClassNormal  MessageClass = "normal"
    ClassPrompt  MessageClass = "prompt"   // Y/n, confirmation requests
    ClassError   MessageClass = "error"    // error messages
    ClassCode    MessageClass = "code"     // code blocks
    ClassSystem  MessageClass = "system"   // system events (window created, etc.)
)

type QuickAction struct {
    Label string // "Yes", "No", "Ctrl+C"
    Keys  string // "y\n", "n\n", "\x03"
}

type Message struct {
    ID          string        // unique ID (window_id + sequence number)
    WindowID    string        // which tmux window this belongs to
    Text        string        // cleaned text content (ANSI stripped)
    RawText     string        // original text with ANSI codes
    Class       MessageClass
    Actions     []QuickAction // quick action buttons (only for ClassPrompt)
    Timestamp   time.Time
}

type WindowStatus string

const (
    StatusWorking  WindowStatus = "working"  // output in last 3s
    StatusWaiting  WindowStatus = "waiting"  // prompt detected, no new output
    StatusIdle     WindowStatus = "idle"     // no output for 30s+
)
```

### Step 2: ANSI Stripping (`internal/parser/ansi.go`)

Implement `func StripANSI(s string) string`:
- Remove all ANSI escape sequences: `\033[...m` (SGR), `\033[...H` (cursor), `\033[...J` (erase), etc.
- Regex pattern: `\x1b\[[0-9;]*[a-zA-Z]` covers most cases
- Also strip `\x1b\]...\x07` (OSC sequences like terminal title changes)
- Also strip `\r` (carriage return) — normalize line endings to `\n` only

### Step 3: Detection Patterns (`internal/parser/patterns.go`)

Define compiled regex patterns:

**Prompt detection** (triggers quick actions):
- `\[Y/n\]`, `\[y/N\]`, `\[yes/no\]`
- `\(y/n\)`, `\(Y/N\)`
- `Continue\?`, `Proceed\?`, `Are you sure\?`
- `Do you want to`, `Would you like to`
- `Press Enter to continue`
- `\[1-9\]` (numbered choice menus)

**Error detection:**
- `^error:`, `^Error:`, `^ERROR:`
- `^fatal:`, `^FATAL:`
- `failed`, `FAILED`
- `panic:` (Go panics)
- `Traceback` (Python)
- `Exception`, `at .+\(.+:\d+\)` (Java/JS stack traces)

**Code block detection:**
- Lines starting with ` ``` ` (markdown code fences)
- 4+ spaces of indentation consistently across 3+ lines

**Prompt line detection** (for message segmentation):
- `[$>%❯#]\s*$` — shell prompts

### Step 4: Parser Logic (`internal/parser/parser.go`)

```go
type Parser struct {
    buffers    map[string]*windowBuffer // per-window accumulator
    output     chan Message
    seqCounter map[string]int
    mu         sync.Mutex
}

type windowBuffer struct {
    windowID    string
    text        strings.Builder
    lastOutput  time.Time
    flushTimer  *time.Timer
    status      WindowStatus
}

func New() *Parser
func (p *Parser) Feed(windowID string, data string)  // called when tmux output arrives
func (p *Parser) Messages() <-chan Message             // read messages
func (p *Parser) Status(windowID string) WindowStatus  // current window status
func (p *Parser) Close()
```

**Feed() logic:**
1. Append data to the window's buffer
2. Reset the flush timer (1.5s)
3. Check if data contains an immediate flush trigger:
   - Confirmation pattern detected → flush immediately, classify as `prompt`
   - Shell prompt detected → flush immediately
4. Update window status to `working`

**Flush logic (called by timer or trigger):**
1. Take accumulated text from buffer
2. Strip ANSI codes for display text
3. Classify the message:
   - Check prompt patterns → `ClassPrompt`, generate QuickActions
   - Check error patterns → `ClassError`
   - Check code patterns → `ClassCode`
   - Default → `ClassNormal`
4. Generate QuickActions for prompts:
   - `[Y/n]` → `[{Label: "Yes", Keys: "y\n"}, {Label: "No", Keys: "n\n"}, {Label: "Ctrl+C", Keys: "\x03"}]`
   - `[y/N]` → same but defaults differ
   - Generic confirm → `[{Label: "Continue", Keys: "\n"}, {Label: "Cancel", Keys: "\x03"}]`
5. Send Message to output channel
6. Update window status to `waiting` if prompt, `idle` if timer-based flush with no recent output

**Status tracking:**
- `working`: output received within last 3 seconds
- `waiting`: last message was a prompt, no new output since
- `idle`: no output for 30+ seconds
- Run a background goroutine that updates statuses every second

## Testing Requirements
- Unit test `StripANSI` with various ANSI sequences (colors, cursor movement, OSC)
- Unit test classification:
  - "Do you want to proceed? [Y/n]" → `ClassPrompt` with Yes/No/Ctrl+C actions
  - "Error: file not found" → `ClassError`
  - "```python\nprint('hello')\n```" → `ClassCode`
  - "normal output text" → `ClassNormal`
- Unit test accumulator flush timing:
  - Feed data, wait 1.5s, verify message is flushed
  - Feed data with prompt pattern, verify immediate flush
- Unit test status tracking:
  - Feed data → status is `working`
  - Feed prompt → status is `waiting`
  - Wait 30s (or mock time) → status is `idle`

## Acceptance Criteria
- [ ] ANSI codes are fully stripped from display text
- [ ] Confirmation prompts are correctly detected and QuickActions generated
- [ ] Error messages are correctly classified
- [ ] Messages are flushed on timeout (1.5s) or trigger (prompt/shell prompt)
- [ ] Window status correctly transitions between working/waiting/idle
- [ ] No goroutine leaks on Close()

## Notes
- The parser must handle partial data — tmux may send output in chunks (mid-word, mid-escape-sequence). The accumulator buffer handles this naturally.
- Don't over-detect errors. Common false positives: "error" in variable names, "failed" in test names. Require the pattern to be at line start or after specific delimiters.
- QuickActions keys should use the exact byte sequences tmux expects via `send-keys`.
- Keep the regex patterns simple and maintainable. Compile them once in `init()`.
