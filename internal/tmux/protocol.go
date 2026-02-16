package tmux

import (
	"errors"
	"strconv"
	"strings"
)

var ErrUnknownLine = errors.New("unknown line format")

func DecodeOctal(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	i := 0
	for i < len(s) {
		if i+3 < len(s) && s[i] == '\\' {
			octal := s[i+1 : i+4]
			if val, err := strconv.ParseInt(octal, 8, 32); err == nil {
				result.WriteByte(byte(val))
				i += 4
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}

	return result.String()
}

func ParseLine(line string) (Event, error) {
	if len(line) == 0 {
		return Event{}, ErrUnknownLine
	}

	if line[0] != '%' {
		return Event{
			Type: EventOutput,
			Data: line,
			Raw:  line,
		}, nil
	}

	if strings.HasPrefix(line, "%output ") {
		return parseOutput(line)
	}

	if strings.HasPrefix(line, "%extended-output ") {
		return parseExtendedOutput(line)
	}

	if strings.HasPrefix(line, "%window-add ") {
		return parseWindowAdd(line)
	}

	if strings.HasPrefix(line, "%window-close ") {
		return parseWindowClose(line)
	}

	if strings.HasPrefix(line, "%window-renamed ") {
		return parseWindowRenamed(line)
	}

	if strings.HasPrefix(line, "%layout-change ") {
		return parseLayoutChange(line)
	}

	if strings.HasPrefix(line, "%begin ") {
		return parseBegin(line)
	}

	if strings.HasPrefix(line, "%end ") {
		return parseEnd(line)
	}

	if strings.HasPrefix(line, "%error ") {
		return parseError(line)
	}

	return Event{}, ErrUnknownLine
}

func parseOutput(line string) (Event, error) {
	rest := strings.TrimPrefix(line, "%output ")
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) < 2 {
		return Event{}, ErrUnknownLine
	}

	paneID := parts[0]
	if len(paneID) > 0 && paneID[0] != '%' {
		return Event{}, ErrUnknownLine
	}

	data := DecodeOctal(parts[1])

	return Event{
		Type:   EventOutput,
		PaneID: paneID,
		Data:   data,
		Raw:    line,
	}, nil
}

func parseExtendedOutput(line string) (Event, error) {
	rest := strings.TrimPrefix(line, "%extended-output ")
	parts := strings.SplitN(rest, " : ", 2)
	if len(parts) < 2 {
		return Event{}, ErrUnknownLine
	}

	headerFields := strings.Fields(parts[0])
	if len(headerFields) < 2 {
		return Event{}, ErrUnknownLine
	}
	paneID := headerFields[0]
	if len(paneID) == 0 || paneID[0] != '%' {
		return Event{}, ErrUnknownLine
	}

	// headerFields[1] is "age" in milliseconds; we don't currently use it.
	if _, err := strconv.Atoi(headerFields[1]); err != nil {
		return Event{}, ErrUnknownLine
	}

	data := DecodeOctal(parts[1])

	return Event{
		Type:   EventOutput,
		PaneID: paneID,
		Data:   data,
		Raw:    line,
	}, nil
}

func parseWindowAdd(line string) (Event, error) {
	rest := strings.TrimPrefix(line, "%window-add ")
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return Event{}, ErrUnknownLine
	}
	windowID := strings.TrimSpace(fields[0])
	if len(windowID) == 0 || windowID[0] != '@' {
		return Event{}, ErrUnknownLine
	}

	return Event{
		Type:     EventWindowAdd,
		WindowID: windowID,
		Raw:      line,
	}, nil
}

func parseWindowClose(line string) (Event, error) {
	rest := strings.TrimPrefix(line, "%window-close ")
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return Event{}, ErrUnknownLine
	}
	windowID := strings.TrimSpace(fields[0])
	if len(windowID) == 0 || windowID[0] != '@' {
		return Event{}, ErrUnknownLine
	}

	return Event{
		Type:     EventWindowClose,
		WindowID: windowID,
		Raw:      line,
	}, nil
}

func parseWindowRenamed(line string) (Event, error) {
	rest := strings.TrimPrefix(line, "%window-renamed ")
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) < 2 {
		return Event{}, ErrUnknownLine
	}

	windowID := strings.TrimSpace(parts[0])
	if len(windowID) == 0 || windowID[0] != '@' {
		return Event{}, ErrUnknownLine
	}

	newName := parts[1]

	return Event{
		Type:     EventWindowRenamed,
		WindowID: windowID,
		Data:     newName,
		Raw:      line,
	}, nil
}

func parseLayoutChange(line string) (Event, error) {
	rest := strings.TrimPrefix(line, "%layout-change ")
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) < 1 {
		return Event{}, ErrUnknownLine
	}

	windowID := strings.TrimSpace(parts[0])
	if len(windowID) == 0 || windowID[0] != '@' {
		return Event{}, ErrUnknownLine
	}

	data := ""
	if len(parts) > 1 {
		data = parts[1]
	}

	return Event{
		Type:     EventLayoutChange,
		WindowID: windowID,
		Data:     data,
		Raw:      line,
	}, nil
}

func parseBegin(line string) (Event, error) {
	rest := strings.TrimPrefix(line, "%begin ")
	parts := strings.Fields(rest)
	if len(parts) < 3 {
		return Event{}, ErrUnknownLine
	}

	return Event{
		Type: EventBegin,
		Data: strings.Join(parts, " "),
		Raw:  line,
	}, nil
}

func parseEnd(line string) (Event, error) {
	rest := strings.TrimPrefix(line, "%end ")
	parts := strings.Fields(rest)
	if len(parts) < 3 {
		return Event{}, ErrUnknownLine
	}

	return Event{
		Type: EventEnd,
		Data: strings.Join(parts, " "),
		Raw:  line,
	}, nil
}

func parseError(line string) (Event, error) {
	rest := strings.TrimPrefix(line, "%error ")
	parts := strings.Fields(rest)
	if len(parts) < 3 {
		return Event{}, ErrUnknownLine
	}

	return Event{
		Type: EventError,
		Data: strings.Join(parts, " "),
		Raw:  line,
	}, nil
}
