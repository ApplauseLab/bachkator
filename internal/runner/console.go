package runner

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"
)

type ConsoleWriter interface {
	TargetOperation(io.Writer, TargetOperationLine)
	CommandOutput(io.Writer, CommandOutputLine)
}

type TargetOperationLine struct {
	Timestamp time.Time
	Label     string
	Status    string
	Operation string
}

type CommandOutputLine struct {
	Timestamp time.Time
	Label     string
	Line      string
}

type defaultConsoleWriter struct{}

func (defaultConsoleWriter) TargetOperation(w io.Writer, line TargetOperationLine) {
	timestamp := line.Timestamp.UTC().Format(time.RFC3339)
	if line.Status == "" {
		_, _ = fmt.Fprintf(w, "%s [%s] %s\n", timestamp, line.Label, line.Operation)
		return
	}
	_, _ = fmt.Fprintf(w, "%s [%s] (%s) %s\n", timestamp, line.Label, line.Status, line.Operation)
}

func (defaultConsoleWriter) CommandOutput(w io.Writer, line CommandOutputLine) {
	timestamp := line.Timestamp.UTC().Format(time.RFC3339)
	_, _ = fmt.Fprintf(w, "%s [%s] %s\n", timestamp, line.Label, line.Line)
}

func (r Runner) consoleWriter() ConsoleWriter {
	return defaultConsoleWriter{}
}

type commandOutputPrefixWriter struct {
	mu      sync.Mutex
	writer  io.Writer
	console ConsoleWriter
	label   string
	buffer  []byte
}

type commandOutputWriter struct {
	stream *commandOutputPrefixWriter
	log    io.Writer
}

func newCommandOutputWriter(
	stream io.Writer,
	log io.Writer,
	console ConsoleWriter,
	label string,
) *commandOutputWriter {
	return &commandOutputWriter{
		stream: newCommandOutputPrefixWriter(stream, console, label),
		log:    log,
	}
}

func (w *commandOutputWriter) Write(p []byte) (int, error) {
	if _, err := w.stream.Write(p); err != nil {
		return 0, err
	}
	if _, err := w.log.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *commandOutputWriter) Flush() {
	w.stream.Flush()
}

func newCommandOutputPrefixWriter(
	writer io.Writer,
	console ConsoleWriter,
	label string,
) *commandOutputPrefixWriter {
	return &commandOutputPrefixWriter{writer: writer, console: console, label: label}
}

func (w *commandOutputPrefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer = append(w.buffer, p...)
	for {
		index := bytes.IndexByte(w.buffer, '\n')
		if index < 0 {
			break
		}
		line := string(w.buffer[:index])
		w.console.CommandOutput(w.writer, CommandOutputLine{
			Timestamp: time.Now().UTC(),
			Label:     w.label,
			Line:      line,
		})
		w.buffer = w.buffer[index+1:]
	}
	return len(p), nil
}

func (w *commandOutputPrefixWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buffer) == 0 {
		return
	}
	w.console.CommandOutput(w.writer, CommandOutputLine{
		Timestamp: time.Now().UTC(),
		Label:     w.label,
		Line:      string(w.buffer),
	})
	w.buffer = nil
}
