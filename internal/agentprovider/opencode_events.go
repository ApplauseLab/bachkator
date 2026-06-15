package agentprovider

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/applauselab/bachkator/internal/evidence"
)

func captureOpenCodeEvents(
	r io.Reader,
	eventsPath string,
	textOut io.Writer,
) (opencodeEventSummary, error) {
	file, err := evidence.CreatePrivateFile(eventsPath)
	if err != nil {
		return opencodeEventSummary{}, err
	}
	defer func() { _ = file.Close() }()
	summary := opencodeEventSummary{}
	reader := bufio.NewReaderSize(r, 64*1024)
	var parseErr error
	rawBytes := 0
	line := []byte{}
	lineTooLong := false
	for {
		fragment, readErr := reader.ReadSlice('\n')
		if len(fragment) > 0 {
			if err := writeOpenCodeFragment(file, fragment, rawBytes); err != nil {
				return summary, err
			}
			rawBytes += len(fragment)
			if rawBytes > maxOpenCodeRawEventBytes && parseErr == nil {
				parseErr = fmt.Errorf(
					"opencode JSONL output exceeded %d bytes",
					maxOpenCodeRawEventBytes,
				)
			}
			if !lineTooLong && len(line)+len(fragment) > maxOpenCodeEventLineBytes {
				parseErr = firstErr(
					parseErr,
					fmt.Errorf("opencode JSONL event exceeded %d bytes", maxOpenCodeEventLineBytes),
				)
				lineTooLong = true
				line = nil
			}
			if !lineTooLong {
				line = append(line, fragment...)
			}
		}
		lineDone := readErr == nil || errors.Is(readErr, io.EOF)
		if lineDone {
			parseErr = consumeOpenCodeLine(line, lineTooLong, &summary, textOut, parseErr)
			line = nil
			lineTooLong = false
		}
		if readErr == nil || errors.Is(readErr, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if parseErr != nil {
			return summary, fmt.Errorf("%w; read opencode JSONL: %v", parseErr, readErr)
		}
		return summary, readErr
	}
	return summary, parseErr
}

func writeOpenCodeFragment(file io.Writer, fragment []byte, rawBytes int) error {
	if rawBytes >= maxOpenCodeRawEventBytes {
		return nil
	}
	remaining := maxOpenCodeRawEventBytes - rawBytes
	writeFragment := fragment
	if len(writeFragment) > remaining {
		writeFragment = writeFragment[:remaining]
	}
	_, err := file.Write(writeFragment)
	return err
}

func consumeOpenCodeLine(
	line []byte,
	lineTooLong bool,
	summary *opencodeEventSummary,
	textOut io.Writer,
	parseErr error,
) error {
	if len(line) == 0 || lineTooLong || strings.TrimSpace(string(line)) == "" {
		return parseErr
	}
	if err := consumeOpenCodeEvent(string(line), summary, textOut); err != nil {
		return firstErr(parseErr, err)
	}
	return parseErr
}

func consumeOpenCodeEvent(line string, summary *opencodeEventSummary, textOut io.Writer) error {
	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return fmt.Errorf("malformed opencode JSONL event: %w", err)
	}
	if sessionID, _ := event["sessionID"].(string); summary.SessionID == "" && sessionID != "" {
		summary.SessionID = sessionID
	}
	if event["type"] == "text" {
		return consumeOpenCodeTextEvent(event, summary, textOut)
	}
	if event["type"] == "tool_use" {
		if text := mirroredOpenCodeToolCall(event); text != "" {
			return mirrorOpenCodeText(summary, textOut, text)
		}
	}
	if event["type"] == "step_finish" {
		consumeOpenCodeStepFinish(event, summary)
	}
	return nil
}

func consumeOpenCodeTextEvent(
	event map[string]any,
	summary *opencodeEventSummary,
	textOut io.Writer,
) error {
	part, _ := event["part"].(map[string]any)
	if part == nil {
		return nil
	}
	text, _ := part["text"].(string)
	return mirrorOpenCodeText(summary, textOut, text)
}

func consumeOpenCodeStepFinish(event map[string]any, summary *opencodeEventSummary) {
	part, _ := event["part"].(map[string]any)
	if part == nil {
		return
	}
	if reason, _ := part["reason"].(string); reason != "" {
		summary.FinishReason = reason
	}
	if tokens, _ := part["tokens"].(map[string]any); tokens != nil {
		summary.Tokens = tokens
	}
	if cost, ok := part["cost"]; ok {
		summary.Cost = cost
	}
}

func mirroredOpenCodeToolCall(event map[string]any) string {
	part, _ := event["part"].(map[string]any)
	if part == nil {
		return ""
	}
	tool, _ := part["tool"].(string)
	if tool == "" {
		tool = "tool"
	}
	return "[opencode tool:" + tool + "]"
}

func mirrorOpenCodeText(summary *opencodeEventSummary, textOut io.Writer, text string) error {
	if text == "" {
		return nil
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if summary.TextBytes+len(text) > maxOpenCodeLogTextBytes {
		return mirrorCappedOpenCodeText(summary, textOut, text)
	}
	summary.TextBytes += len(text)
	if textOut != nil {
		_, _ = io.WriteString(textOut, sanitizeProviderText(text))
	}
	return nil
}

func mirrorCappedOpenCodeText(
	summary *opencodeEventSummary,
	textOut io.Writer,
	text string,
) error {
	remaining := maxOpenCodeLogTextBytes - summary.TextBytes
	if remaining > 0 && textOut != nil {
		_, _ = io.WriteString(textOut, sanitizeProviderText(text[:remaining]))
	}
	summary.TextBytes = maxOpenCodeLogTextBytes
	return fmt.Errorf("opencode text output exceeded %d bytes", maxOpenCodeLogTextBytes)
}

func sanitizeProviderText(text string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\t':
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, text)
}

func firstErr(current error, next error) error {
	if current != nil {
		return current
	}
	return next
}
