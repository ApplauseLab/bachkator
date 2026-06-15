package runner

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommandOutputPrefixWriterFlushesLongPartialLines(t *testing.T) {
	var out bytes.Buffer
	writer := newCommandOutputPrefixWriter(&out, defaultConsoleWriter{}, "target", nil)

	input := strings.Repeat("x", maxCommandOutputLineBytes+1)
	if n, err := writer.Write([]byte(input)); err != nil || n != len(input) {
		t.Fatalf("Write() = %d, %v; want %d, nil", n, err, len(input))
	}

	if got := len(writer.buffer); got != 1 {
		t.Fatalf("buffer length = %d, want 1", got)
	}
	if !strings.Contains(out.String(), strings.Repeat("x", maxCommandOutputLineBytes)) {
		t.Fatalf("output did not include flushed long line chunk")
	}
}
