package jsonrpcstdio

import (
	"bufio"
	"bytes"
	"testing"
)

func TestWriteAndReadMessage(t *testing.T) {
	var buffer bytes.Buffer
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"test"}`)
	if err := WriteMessage(&buffer, payload); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMessage(bufio.NewReader(&buffer))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload = %s, want %s", got, payload)
	}
}

func TestReadMessageRejectsMissingContentLength(t *testing.T) {
	_, err := ReadMessage(bufio.NewReader(bytes.NewBufferString("\r\n{}")))
	if err == nil {
		t.Fatal("expected missing Content-Length error")
	}
}

func TestReadMessageRejectsDuplicateContentLength(t *testing.T) {
	input := "Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}"
	_, err := ReadMessage(bufio.NewReader(bytes.NewBufferString(input)))
	if err == nil {
		t.Fatal("expected duplicate Content-Length error")
	}
}

func TestReadMessageRejectsOversizedPayload(t *testing.T) {
	input := "Content-Length: 8388609\r\n\r\n"
	_, err := ReadMessage(bufio.NewReader(bytes.NewBufferString(input)))
	if err == nil {
		t.Fatal("expected oversized payload error")
	}
}

func TestWriteMessageRejectsOversizedPayload(t *testing.T) {
	err := WriteMessage(&bytes.Buffer{}, bytes.Repeat([]byte("x"), MaxContentLength+1))
	if err == nil {
		t.Fatal("expected oversized payload error")
	}
}
