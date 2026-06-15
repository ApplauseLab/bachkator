package factorydaemon

import (
	"context"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/config"
	factorypkg "github.com/applauselab/bachkator/internal/factory"
	"github.com/applauselab/bachkator/pkg/triggerprotocol"
)

func TestTriggerPollerPollEnqueuesItem(t *testing.T) {
	ctx := context.Background()
	poller, handler, cleanup := newTestPoller(t, "test", "default")
	defer cleanup()
	handler.result = triggerprotocol.PollResult{
		Cursor: "c1",
		Items: []triggerprotocol.PollItem{
			{
				Source: triggerprotocol.ItemSource{Type: "issue", ID: "42"},
				Title:  "Fix bug",
				Body:   "body text",
			},
		},
	}

	poller.poll(ctx)

	cursor, err := poller.service.Backend.Factory.GetTriggerCursor(ctx, "test", "gh")
	if err != nil {
		t.Fatalf("get cursor: %v", err)
	}
	if cursor.Cursor != "c1" {
		t.Errorf("cursor = %q, want c1", cursor.Cursor)
	}
	if cursor.LastAckAt.IsZero() {
		t.Errorf("expected ack timestamp")
	}
	if !cursor.LastNackAt.IsZero() {
		t.Errorf("unexpected nack timestamp")
	}

	select {
	case ack := <-handler.acks:
		if ack.Cursor != "c1" {
			t.Errorf("ack cursor = %q, want c1", ack.Cursor)
		}
		if len(ack.SourceIDs) != 1 || ack.SourceIDs[0] != "42" {
			t.Errorf("ack source ids = %v, want [42]", ack.SourceIDs)
		}
	case <-time.After(time.Second):
		t.Fatal("expected ack")
	}

	items, err := poller.service.Backend.Factory.List(
		ctx,
		backend.FactoryWorkItemQuery{Factory: "test"},
	)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Title != "Fix bug" {
		t.Errorf("title = %q, want Fix bug", items[0].Title)
	}
	if items[0].Workflow != "default" {
		t.Errorf("workflow = %q, want default", items[0].Workflow)
	}
}

func TestTriggerPollerRouteUsesLabel(t *testing.T) {
	ctx := context.Background()
	poller, handler, cleanup := newTestPoller(t, "test", "")
	defer cleanup()
	poller.service.Factory.Workflows = []*config.FactoryWorkflow{
		{Name: "default"},
		{Name: "urgent"},
	}
	poller.trigger.Route = []*config.ProviderRoute{
		{Label: "urgent", Workflow: "urgent"},
	}
	poller.defaultWorkflow = ""
	handler.result = triggerprotocol.PollResult{
		Cursor: "c1",
		Items: []triggerprotocol.PollItem{
			{
				Source: triggerprotocol.ItemSource{Type: "issue", ID: "1"},
				Title:  "Urgent bug",
				Labels: []string{"urgent"},
			},
		},
	}

	poller.poll(ctx)

	items, err := poller.service.Backend.Factory.List(
		ctx,
		backend.FactoryWorkItemQuery{Factory: "test"},
	)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Workflow != "urgent" {
		t.Errorf("workflow = %q, want urgent", items[0].Workflow)
	}
}

func TestTriggerPollerNackOnIntakeError(t *testing.T) {
	ctx := context.Background()
	poller, handler, cleanup := newTestPoller(t, "test", "default")
	defer cleanup()
	handler.result = triggerprotocol.PollResult{
		Cursor: "c1",
		Items: []triggerprotocol.PollItem{
			{
				Source: triggerprotocol.ItemSource{Type: "issue", ID: "1"},
				Title:  "   ",
			},
		},
	}

	poller.poll(ctx)

	cursor, err := poller.service.Backend.Factory.GetTriggerCursor(ctx, "test", "gh")
	if err != nil {
		t.Fatalf("get cursor: %v", err)
	}
	if cursor.LastNackAt.IsZero() {
		t.Errorf("expected nack timestamp")
	}
	if !cursor.LastAckAt.IsZero() {
		t.Errorf("unexpected ack timestamp")
	}

	select {
	case nack := <-handler.nacks:
		if nack.Cursor != "c1" {
			t.Errorf("nack cursor = %q, want c1", nack.Cursor)
		}
	case <-time.After(time.Second):
		t.Fatal("expected nack")
	}

	items, err := poller.service.Backend.Factory.List(
		ctx,
		backend.FactoryWorkItemQuery{Factory: "test"},
	)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("items = %d, want 0", len(items))
	}
}

type fakeTriggerHandler struct {
	mu     sync.Mutex
	result triggerprotocol.PollResult
	acks   chan triggerprotocol.AckParams
	nacks  chan triggerprotocol.NackParams
}

func (h *fakeTriggerHandler) Handshake(
	_ context.Context,
	_ triggerprotocol.HandshakeParams,
) (triggerprotocol.HandshakeResult, error) {
	return triggerprotocol.HandshakeResult{
		Protocol: triggerprotocol.ProtocolVersion,
		Provider: "fake",
		Capabilities: []triggerprotocol.Capability{
			triggerprotocol.CapabilityPoll,
			triggerprotocol.CapabilityAck,
			triggerprotocol.CapabilityNack,
		},
	}, nil
}

func (h *fakeTriggerHandler) Poll(
	_ context.Context,
	_ triggerprotocol.PollParams,
) (triggerprotocol.PollResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.result, nil
}

func (h *fakeTriggerHandler) Ack(_ context.Context, params triggerprotocol.AckParams) error {
	h.acks <- params
	return nil
}

func (h *fakeTriggerHandler) Nack(_ context.Context, params triggerprotocol.NackParams) error {
	h.nacks <- params
	return nil
}

func newTestPoller(
	t *testing.T,
	factoryName, defaultWorkflow string,
) (*triggerPoller, *fakeTriggerHandler, func()) {
	t.Helper()
	statePath := filepath.Join(t.TempDir(), "state.db")
	client := backend.NewClient(statePath)

	svc := Service{
		ConfigProject: &config.Project{Root: t.TempDir()},
		Factory: &config.Factory{
			Name:      factoryName,
			Workflows: []*config.FactoryWorkflow{{Name: defaultWorkflow}},
		},
		Backend: client,
		Stderr:  io.Discard,
	}
	factorySvc := factorypkg.Service{
		Root:  svc.ConfigProject.Root,
		Queue: factorypkg.BackendQueue{Client: &client.Factory},
	}

	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	clientConn := triggerprotocol.NewClient(clientReader, clientWriter)

	handler := &fakeTriggerHandler{
		acks:  make(chan triggerprotocol.AckParams, 10),
		nacks: make(chan triggerprotocol.NackParams, 10),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = triggerprotocol.Serve(context.Background(), serverReader, serverWriter, handler)
	}()

	trigger := &config.FactoryProviderTrigger{
		Name:         "gh",
		Command:      []string{"true"},
		PollInterval: "1h",
	}
	poller := &triggerPoller{
		service:         svc,
		factoryService:  factorySvc,
		factory:         factoryName,
		trigger:         trigger,
		defaultWorkflow: defaultWorkflow,
		session:         &triggerSession{client: clientConn},
	}

	cleanup := func() {
		_ = clientWriter.Close()
		_ = serverWriter.Close()
		wg.Wait()
	}
	return poller, handler, cleanup
}
