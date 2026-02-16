package parser

import "regexp"

var (
	ansiCSI      *regexp.Regexp
	ansiOSC      *regexp.Regexp
	ansiDCS      *regexp.Regexp
	ansiPM       *regexp.Regexp
	ansiAPC      *regexp.Regexp
	ansiOldTitle *regexp.Regexp
	ansiCharset  *regexp.Regexp
	ansiKeypad   *regexp.Regexp
	ansiSingle   *regexp.Regexp
)

func init() {
	ansiCSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	ansiOSC = regexp.MustCompile(`\x1b\].*?(?:\x07|\x1b\\)`)
	ansiDCS = regexp.MustCompile(`\x1bP.*?\x1b\\`)
	ansiPM = regexp.MustCompile(`\x1b\^.*?\x1b\\`)
	ansiAPC = regexp.MustCompile(`\x1b_.*?\x1b\\`)
	ansiOldTitle = regexp.MustCompile(`\x1bk.*?\x1b\\`)
	ansiCharset = regexp.MustCompile(`\x1b[()][0-9A-Za-z]`)
	ansiKeypad = regexp.MustCompile(`\x1b[=>]`)
	ansiSingle = regexp.MustCompile(`\x1b.`)
}

func StripANSI(s string) string {
	s = ansiCSI.ReplaceAllString(s, "")
	s = ansiOSC.ReplaceAllString(s, "")
	s = ansiDCS.ReplaceAllString(s, "")
	s = ansiPM.ReplaceAllString(s, "")
	s = ansiAPC.ReplaceAllString(s, "")
	s = ansiOldTitle.ReplaceAllString(s, "")
	s = ansiCharset.ReplaceAllString(s, "")
	s = ansiKeypad.ReplaceAllString(s, "")
	s = ansiSingle.ReplaceAllString(s, "")

	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\r' {
			continue
		}
		if ch == '\b' {
			if len(result) > 0 {
				result = result[:len(result)-1]
			}
			continue
		}
		// Remove remaining control bytes except line breaks and tabs.
		if (ch < 0x20 || ch == 0x7f) && ch != '\n' && ch != '\t' {
			continue
		}
		result = append(result, ch)
	}
	return string(result)
}
