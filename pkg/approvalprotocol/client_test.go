package approvalprotocol

import (
	"context"
	"io"
	"sync"
	"testing"
)

func TestClientRoundTripThroughServer(t *testing.T) {
	handler := &stubHandler{
		records: []ApprovalRecord{
			{
				SourceType: "github_issue",
				SourceID:   "42",
				Phase:      "plan",
				Approver:   "kris",
				Reason:     "looks good",
			},
		},
	}
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = Serve(context.Background(), serverReader, serverWriter, handler)
	}()

	client := NewClient(clientReader, clientWriter)
	handshake, err := client.Handshake(context.Background(), HandshakeParams{
		Protocol: ProtocolVersion,
		Factory:  "sldc",
		Approval: "github",
	})
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	if handshake.Protocol != ProtocolVersion {
		t.Errorf("protocol = %q, want %q", handshake.Protocol, ProtocolVersion)
	}
	if handshake.Provider != "fake" {
		t.Errorf("provider = %q, want fake", handshake.Provider)
	}

	result, err := client.Poll(context.Background(), PollParams{
		Config: map[string]string{"items_path": "x.json"},
	})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(result.Approvals) != 1 {
		t.Fatalf("approvals = %d, want 1", len(result.Approvals))
	}
	record := result.Approvals[0]
	if record.SourceType != "github_issue" || record.SourceID != "42" {
		t.Errorf("source = %q/%q, want github_issue/42", record.SourceType, record.SourceID)
	}
	if record.Phase != "plan" || record.Approver != "kris" {
		t.Errorf("record = %+v, want plan/kris", record)
	}

	if err := clientWriter.Close(); err != nil {
		t.Errorf("close client writer: %v", err)
	}
	if err := serverWriter.Close(); err != nil {
		t.Errorf("close server writer: %v", err)
	}
	wg.Wait()
}

type stubHandler struct {
	records []ApprovalRecord
}

func (h *stubHandler) Handshake(
	_ context.Context,
	params HandshakeParams,
) (HandshakeResult, error) {
	if params.Protocol != ProtocolVersion {
		return HandshakeResult{}, NewError(
			ErrorUnsupportedProtocol,
			"unsupported protocol "+params.Protocol,
		)
	}
	return HandshakeResult{
		Protocol:     ProtocolVersion,
		Provider:     "fake",
		Version:      "v1",
		Capabilities: []Capability{CapabilityPoll},
	}, nil
}

func (h *stubHandler) Poll(_ context.Context, _ PollParams) (PollResult, error) {
	return PollResult{Approvals: h.records}, nil
}
