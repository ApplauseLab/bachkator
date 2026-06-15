package jsonrpcstdio

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	contentLengthHeader = "Content-Length:"
	MaxHeaderBytes      = 16 * 1024
	MaxContentLength    = 8 * 1024 * 1024
)

func ReadMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	headerBytes := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		headerBytes += len(line)
		if headerBytes > MaxHeaderBytes {
			return nil, fmt.Errorf("message headers exceed %d bytes", MaxHeaderBytes)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if value, ok := strings.CutPrefix(line, contentLengthHeader); ok {
			if length >= 0 {
				return nil, fmt.Errorf("duplicate Content-Length header")
			}
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || parsed < 0 {
				return nil, fmt.Errorf("invalid Content-Length header %q", line)
			}
			if parsed > MaxContentLength {
				return nil, fmt.Errorf("Content-Length %d exceeds max %d", parsed, MaxContentLength)
			}
			length = parsed
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func WriteMessage(w io.Writer, payload []byte) error {
	if !utf8.Valid(payload) {
		return fmt.Errorf("payload is not valid utf-8")
	}
	if len(payload) > MaxContentLength {
		return fmt.Errorf("payload length %d exceeds max %d", len(payload), MaxContentLength)
	}
	_, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
	return err
}
