package triggerprotocol

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"
	"testing"
)

type testHandler struct {
	handshake HandshakeResult
	poll      PollResult
	ackCursor string
	nack      *NackParams
}

func (h *testHandler) Handshake(
	_ context.Context,
	params HandshakeParams,
) (HandshakeResult, error) {
	if params.Protocol != ProtocolVersion {
		return HandshakeResult{}, NewError(ErrorUnsupportedProtocol, "protocol "+params.Protocol)
	}
	return h.handshake, nil
}

func (h *testHandler) Poll(_ context.Context, params PollParams) (PollResult, error) {
	_ = params
	return h.poll, nil
}

func (h *testHandler) Ack(_ context.Context, params AckParams) error {
	h.ackCursor = params.Cursor
	return nil
}

func (h *testHandler) Nack(_ context.Context, params NackParams) error {
	h.nack = &params
	return nil
}

func TestClientServerHandshakeAndPoll(t *testing.T) {
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()

	handler := &testHandler{
		handshake: HandshakeResult{
			Protocol:     ProtocolVersion,
			Provider:     "test",
			Version:      "1.0.0",
			Capabilities: []Capability{CapabilityPoll, CapabilityAck, CapabilityNack},
		},
		poll: PollResult{
			Cursor: "cursor-2",
			Items: []PollItem{{
				Source: ItemSource{Type: "test", ID: "1"},
				Title:  "Hello",
			}},
		},
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- Serve(context.Background(), serverIn, serverOut, handler)
	}()

	client := NewClient(clientIn, clientOut)
	hs, err := client.Handshake(context.Background(), HandshakeParams{
		Protocol: ProtocolVersion,
		Factory:  "sldc",
		Trigger:  "test",
	})
	if err != nil {
		t.Fatalf("Handshake() error = %v", err)
	}
	if hs.Provider != "test" {
		t.Fatalf("provider = %q, want test", hs.Provider)
	}

	poll, err := client.Poll(context.Background(), PollParams{Cursor: "cursor-1"})
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if poll.Cursor != "cursor-2" {
		t.Fatalf("cursor = %q, want cursor-2", poll.Cursor)
	}
	if len(poll.Items) != 1 || poll.Items[0].Title != "Hello" {
		t.Fatalf("items = %v, want one Hello", poll.Items)
	}

	if err := client.Ack(context.Background(), AckParams{Cursor: "cursor-2"}); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
	if handler.ackCursor != "cursor-2" {
		t.Fatalf("ack cursor = %q, want cursor-2", handler.ackCursor)
	}

	_ = clientOut.Close()
	if err := <-serverErr; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}

func TestServerRejectsUnknownMethod(t *testing.T) {
	var buf bytes.Buffer
	request := `{"jsonrpc":"2.0","id":1,"method":"trigger.unknown","params":{}}`
	if err := writeRawMessage(&buf, []byte(request)); err != nil {
		t.Fatal(err)
	}

	handler := &testHandler{}
	if err := Serve(context.Background(), &buf, io.Discard, handler); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}

func TestClientReturnsDomainError(t *testing.T) {
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- Serve(context.Background(), serverIn, serverOut, &testHandler{})
	}()

	client := NewClient(clientIn, clientOut)
	_, err := client.Handshake(context.Background(), HandshakeParams{Protocol: "wrong"})
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
	if !strings.Contains(err.Error(), "unsupported_protocol") {
		t.Fatalf("error = %v, want unsupported_protocol", err)
	}

	_ = clientOut.Close()
	_ = serverIn.Close()
	<-serverErr
}

func writeRawMessage(w io.Writer, payload []byte) error {
	header := "Content-Length: " + strconv.Itoa(len(payload)) + "\r\n\r\n"
	if _, err := w.Write([]byte(header)); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}
