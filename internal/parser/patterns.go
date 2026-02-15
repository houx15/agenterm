package parser

import "regexp"

var (
	PromptConfirmPattern  *regexp.Regexp
	PromptQuestionPattern *regexp.Regexp
	PromptShellPattern    *regexp.Regexp
	ErrorPattern          *regexp.Regexp
	CodeFencePattern      *regexp.Regexp
	CodeIndentPattern     *regexp.Regexp
	NumberedChoicePattern *regexp.Regexp
)

func init() {
	PromptConfirmPattern = regexp.MustCompile(`(?i)\[(Y/n|y/N|yes/no)\]|\(y/n\)|\(Y/N\)`)
	PromptQuestionPattern = regexp.MustCompile(`(?i)(Continue\?|Proceed\?|Are you sure\?|Do you want to|Would you like to|Press Enter to continue)`)
	NumberedChoicePattern = regexp.MustCompile(`^\s*\d+[\.\)]\s+.+\s*$`)
	PromptShellPattern = regexp.MustCompile(`[$>%❯#]\s*$`)
	ErrorPattern = regexp.MustCompile(`(?m)^(?:(?:error|Error|ERROR|fatal|FATAL|panic):)|(?:failed|FAILED)|(?:Traceback)|(?:Exception.*at\s+.+\(.+:\d+\))`)
	CodeFencePattern = regexp.MustCompile("^```")
	CodeIndentPattern = regexp.MustCompile(`^( {4,}|\t)`)
}
