package parser

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type windowBuffer struct {
	windowID   string
	text       strings.Builder
	lastOutput time.Time
	flushTimer *time.Timer
	status     WindowStatus
}

type Parser struct {
	buffers    map[string]*windowBuffer
	output     chan Message
	seqCounter map[string]int
	mu         sync.Mutex
	done       chan struct{}
	wg         sync.WaitGroup
}

func New() *Parser {
	p := &Parser{
		buffers:    make(map[string]*windowBuffer),
		output:     make(chan Message, 100),
		seqCounter: make(map[string]int),
		done:       make(chan struct{}),
	}
	p.wg.Add(1)
	go p.statusUpdater()
	return p
}

func (p *Parser) Feed(windowID string, data string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	buf, exists := p.buffers[windowID]
	if !exists {
		buf = &windowBuffer{
			windowID: windowID,
			status:   StatusWorking,
		}
		p.buffers[windowID] = buf
	}

	buf.text.WriteString(data)
	buf.lastOutput = time.Now()
	buf.status = StatusWorking

	if buf.flushTimer != nil {
		buf.flushTimer.Stop()
	}

	cleanText := StripANSI(buf.text.String())
	immediateFlush := false
	var classification MessageClass = ClassNormal
	var actions []QuickAction

	if PromptConfirmPattern.MatchString(cleanText) || PromptQuestionPattern.MatchString(cleanText) {
		immediateFlush = true
		classification = ClassPrompt
		actions = generateQuickActions(cleanText)
	} else if PromptShellPattern.MatchString(cleanText) {
		immediateFlush = true
	}

	if immediateFlush {
		p.flushBufferLocked(buf, classification, actions)
	} else {
		buf.flushTimer = time.AfterFunc(1500*time.Millisecond, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if buf.text.Len() > 0 {
				p.flushBufferLocked(buf, ClassNormal, nil)
			}
		})
	}
}

func (p *Parser) flushBufferLocked(buf *windowBuffer, forcedClass MessageClass, forcedActions []QuickAction) {
	if buf.text.Len() == 0 {
		return
	}

	rawText := buf.text.String()
	cleanText := StripANSI(rawText)
	buf.text.Reset()

	class := forcedClass
	actions := forcedActions

	if class == ClassNormal {
		class, actions = p.classify(cleanText)
	}

	p.seqCounter[buf.windowID]++
	id := fmt.Sprintf("%s-%d", buf.windowID, p.seqCounter[buf.windowID])

	msg := Message{
		ID:        id,
		WindowID:  buf.windowID,
		Text:      cleanText,
		RawText:   rawText,
		Class:     class,
		Actions:   actions,
		Timestamp: time.Now(),
	}

	select {
	case p.output <- msg:
	default:
	}

	if class == ClassPrompt {
		buf.status = StatusWaiting
	}
}

func (p *Parser) classify(text string) (MessageClass, []QuickAction) {
	if PromptConfirmPattern.MatchString(text) || PromptQuestionPattern.MatchString(text) || hasNumberedChoices(text) {
		actions := generateQuickActions(text)
		return ClassPrompt, actions
	}

	if ErrorPattern.MatchString(text) {
		return ClassError, nil
	}

	if CodeFencePattern.MatchString(text) || hasCodeIndent(text) {
		return ClassCode, nil
	}

	return ClassNormal, nil
}

func hasNumberedChoices(text string) bool {
	lines := strings.Split(text, "\n")
	count := 0
	for _, line := range lines {
		if NumberedChoicePattern.MatchString(line) {
			count++
		}
	}
	return count >= 2
}

func hasCodeIndent(text string) bool {
	lines := strings.Split(text, "\n")
	indentedCount := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		if CodeIndentPattern.MatchString(line) {
			indentedCount++
		}
	}
	return indentedCount >= 3
}

func generateQuickActions(text string) []QuickAction {
	if strings.Contains(text, "[Y/n]") || strings.Contains(text, "[Y/N]") {
		return []QuickAction{
			{Label: "Yes", Keys: "y\n"},
			{Label: "No", Keys: "n\n"},
			{Label: "Ctrl+C", Keys: "\x03"},
		}
	}
	if strings.Contains(text, "[y/N]") {
		return []QuickAction{
			{Label: "Yes", Keys: "y\n"},
			{Label: "No", Keys: "n\n"},
			{Label: "Ctrl+C", Keys: "\x03"},
		}
	}
	return []QuickAction{
		{Label: "Continue", Keys: "\n"},
		{Label: "Cancel", Keys: "\x03"},
	}
}

func (p *Parser) Messages() <-chan Message {
	return p.output
}

func (p *Parser) Status(windowID string) WindowStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	buf, exists := p.buffers[windowID]
	if !exists {
		return StatusIdle
	}
	return buf.status
}

func (p *Parser) statusUpdater() {
	defer p.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.updateStatuses()
		}
	}
}

func (p *Parser) updateStatuses() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for _, buf := range p.buffers {
		if buf.status == StatusWaiting {
			continue
		}
		if now.Sub(buf.lastOutput) > 30*time.Second {
			buf.status = StatusIdle
		} else if now.Sub(buf.lastOutput) > 3*time.Second {
			if buf.status != StatusWaiting {
				buf.status = StatusIdle
			}
		}
	}
}

func (p *Parser) Close() {
	close(p.done)
	p.wg.Wait()

	p.mu.Lock()
	for _, buf := range p.buffers {
		if buf.flushTimer != nil {
			buf.flushTimer.Stop()
		}
		if buf.text.Len() > 0 {
			p.flushBufferLocked(buf, ClassNormal, nil)
		}
	}
	p.mu.Unlock()

	close(p.output)
}
