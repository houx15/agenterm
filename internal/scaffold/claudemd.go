package scaffold

import (
	"strings"
)

// ContextFileName returns the appropriate context file name for the agent type.
func ContextFileName(agentType string) string {
	switch strings.ToLower(agentType) {
	case "claude", "claude-code":
		return "CLAUDE.md"
	default:
		return "AGENTS.md"
	}
}

// GenerateContextFile generates a CLAUDE.md or AGENTS.md file content
// for a given agent type, project context, and task specification.
func GenerateContextFile(agentType string, projectName string, contextTemplate string, task BlueprintTask) string {
	var b strings.Builder

	filename := ContextFileName(agentType)

	b.WriteString("# ")
	b.WriteString(filename)
	b.WriteString("\n\n")

	b.WriteString("## Project\n\n")
	b.WriteString(projectName)
	b.WriteString("\n\n")

	b.WriteString("## Task\n\n")
	b.WriteString("**")
	b.WriteString(task.Title)
	b.WriteString("**\n\n")
	if task.Description != "" {
		b.WriteString(task.Description)
		b.WriteString("\n\n")
	}

	if len(task.CompletionCriteria) > 0 {
		b.WriteString("## Completion Criteria\n\n")
		for _, criterion := range task.CompletionCriteria {
			b.WriteString("- ")
			b.WriteString(criterion)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Rules\n\n")
	b.WriteString("- Do not ask for user input; operate autonomously\n")
	b.WriteString("- If blocked, write a BLOCKED.md file describing the issue and stop\n")
	b.WriteString("- Run tests after making changes\n")
	b.WriteString("- Commit frequently with descriptive messages\n")
	b.WriteString("- Do not modify files outside the scope of this task\n")
	b.WriteString("\n")

	if contextTemplate != "" {
		b.WriteString("## Conventions\n\n")
		b.WriteString(contextTemplate)
		b.WriteString("\n")
	}

	return b.String()
}
