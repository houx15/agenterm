package parser

import "regexp"

var (
	ansiCSI   *regexp.Regexp
	ansiOSC   *regexp.Regexp
	ansiOther *regexp.Regexp
)

func init() {
	ansiCSI = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	ansiOSC = regexp.MustCompile(`\x1b\].*?(?:\x07|\x1b\\)`)
	ansiOther = regexp.MustCompile(`\x1b[()][AB012]`)
}

func StripANSI(s string) string {
	s = ansiCSI.ReplaceAllString(s, "")
	s = ansiOSC.ReplaceAllString(s, "")
	s = ansiOther.ReplaceAllString(s, "")
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' {
			continue
		}
		result = append(result, s[i])
	}
	return string(result)
}
