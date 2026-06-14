package factorydaemon

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/config"
	factorypkg "github.com/applauselab/bachkator/internal/factory"
	"github.com/applauselab/bachkator/pkg/approvalprotocol"
)

func TestApprovalPollerRecordsBySourceIdentity(t *testing.T) {
	ctx := context.Background()
	poller, handler, workItemID, cleanup := newTestApprovalPoller(t)
	defer cleanup()
	handler.records = []approvalprotocol.ApprovalRecord{
		{
			SourceType: "github_issue",
			SourceID:   "42",
			Phase:      "plan",
			Approver:   "kris",
			Reason:     "ship it",
			Metadata: map[string]string{
				"issue_url": "https://example.test/issues/42",
			},
		},
	}

	poller.poll(ctx)

	approvals, err := poller.service.Backend.Factory.ListApprovals(ctx, workItemID)
	if err != nil {
		t.Fatalf("list approvals: %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("approvals = %d, want 1", len(approvals))
	}
	approval := approvals[0]
	if approval.Phase != "plan" {
		t.Errorf("phase = %q, want plan", approval.Phase)
	}
	if approval.Approver != "kris" {
		t.Errorf("approver = %q, want kris", approval.Approver)
	}
	if approval.ApproverSource != approverSourceProvider {
		t.Errorf("approver source = %q, want %q", approval.ApproverSource, approverSourceProvider)
	}
	if approval.PlanPath != "plans/factory-request.md" {
		t.Errorf("plan path = %q, want plans/factory-request.md", approval.PlanPath)
	}
	if approval.PlanHash == "" {
		t.Error("expected plan hash evidence")
	}
	if approval.Metadata["issue_url"] != "https://example.test/issues/42" {
		t.Errorf("metadata = %v, want issue_url preserved", approval.Metadata)
	}

	poller.poll(ctx)

	approvals, err = poller.service.Backend.Factory.ListApprovals(ctx, workItemID)
	if err != nil {
		t.Fatalf("re-list approvals: %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("redelivered approvals = %d, want idempotent 1", len(approvals))
	}
}

func TestApprovalPollerRecordsByWorkItemID(t *testing.T) {
	ctx := context.Background()
	poller, handler, workItemID, cleanup := newTestApprovalPoller(t)
	defer cleanup()
	handler.records = []approvalprotocol.ApprovalRecord{
		{WorkItemID: workItemID, Phase: "plan"},
	}

	poller.poll(ctx)

	approvals, err := poller.service.Backend.Factory.ListApprovals(ctx, workItemID)
	if err != nil {
		t.Fatalf("list approvals: %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("approvals = %d, want 1", len(approvals))
	}
}

func TestApprovalPollerRejectsInvalidRecords(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		record approvalprotocol.ApprovalRecord
	}{
		{
			name:   "missing phase",
			record: approvalprotocol.ApprovalRecord{SourceType: "github_issue", SourceID: "42"},
		},
		{
			name: "unknown source",
			record: approvalprotocol.ApprovalRecord{
				SourceType: "gitlab_issue",
				SourceID:   "99",
				Phase:      "plan",
			},
		},
		{
			name: "ungated phase",
			record: approvalprotocol.ApprovalRecord{
				SourceType: "github_issue",
				SourceID:   "42",
				Phase:      "implement",
			},
		},
		{
			name: "both identity modes",
			record: approvalprotocol.ApprovalRecord{
				WorkItemID: "item",
				SourceType: "github_issue",
				SourceID:   "42",
				Phase:      "plan",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poller, handler, workItemID, cleanup := newTestApprovalPoller(t)
			defer cleanup()
			handler.records = []approvalprotocol.ApprovalRecord{tt.record}

			poller.poll(ctx)

			approvals, err := poller.service.Backend.Factory.ListApprovals(ctx, workItemID)
			if err != nil {
				t.Fatalf("list approvals: %v", err)
			}
			if len(approvals) != 0 {
				t.Fatalf("approvals = %d, want 0", len(approvals))
			}
		})
	}
}

type fakeApprovalHandler struct {
	mu      sync.Mutex
	records []approvalprotocol.ApprovalRecord
}

func (h *fakeApprovalHandler) Handshake(
	_ context.Context,
	params approvalprotocol.HandshakeParams,
) (approvalprotocol.HandshakeResult, error) {
	return approvalprotocol.HandshakeResult{
		Protocol:     params.Protocol,
		Provider:     "fake",
		Version:      "v1",
		Capabilities: []approvalprotocol.Capability{approvalprotocol.CapabilityPoll},
	}, nil
}

func (h *fakeApprovalHandler) Poll(
	_ context.Context,
	_ approvalprotocol.PollParams,
) (approvalprotocol.PollResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return approvalprotocol.PollResult{Approvals: h.records}, nil
}

func newTestApprovalPoller(
	t *testing.T,
) (*approvalPoller, *fakeApprovalHandler, string, func()) {
	t.Helper()
	root := t.TempDir()
	statePath := filepath.Join(root, ".bach", "state.db")
	client := backend.NewClient(statePath)

	workflows := []*config.FactoryWorkflow{
		{
			Name: "default",
			Plan: []*config.FactoryPlanPhase{
				{AgentTemplate: "t", Path: "plans/factory-request.md"},
			},
			Implement: []*config.FactoryImplementPhase{
				{AgentTemplate: "t"},
			},
		},
	}
	svc := Service{
		ConfigProject: &config.Project{Root: root},
		Factory:       &config.Factory{Name: "test", Workflows: workflows},
		Backend:       client,
		Stderr:        io.Discard,
	}
	factorySvc := factorypkg.Service{
		Root:  root,
		Queue: factorypkg.BackendQueue{Client: &client.Factory},
	}

	result, err := factorySvc.ProviderIntake(context.Background(), factorypkg.ProviderIntakeOptions{
		Factory:    "test",
		Trigger:    "gh",
		Workflow:   "default",
		SourceType: "github_issue",
		SourceID:   "42",
		Title:      "Fix bug",
	})
	if err != nil {
		t.Fatalf("provider intake: %v", err)
	}
	item, ok, err := client.Factory.Get(context.Background(), "test", result.WorkItemID)
	if err != nil || !ok {
		t.Fatalf("get work item: ok=%v err=%v", ok, err)
	}
	attemptID := firstAttemptID(item)
	writeTestPlanFile(t, root, "plans/factory-request.md")

	err = client.Factory.UpdatePhase(context.Background(), backend.FactoryWorkItemPhase{
		WorkItemID: item.ID,
		AttemptID:  attemptID,
		PhaseKey:   "plan",
		Status:     PhaseWaitingApproval,
		UpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("mark waiting approval: %v", err)
	}

	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()

	handler := &fakeApprovalHandler{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = approvalprotocol.Serve(context.Background(), serverReader, serverWriter, handler)
	}()

	provider := &config.FactoryApprovalProvider{
		Name:         "github",
		Command:      []string{"true"},
		PollInterval: "1h",
	}
	poller := &approvalPoller{
		service:        svc,
		factoryService: factorySvc,
		factory:        "test",
		provider:       provider,
		session: newProviderSession(
			provider.Name,
			provider.Command,
			root,
			func(_ io.Reader, _ io.Writer) *approvalprotocol.Client {
				return approvalprotocol.NewClient(clientReader, clientWriter)
			},
			func(context.Context, *approvalprotocol.Client) error { return nil },
		),
	}

	cleanup := func() {
		_ = clientWriter.Close()
		_ = serverWriter.Close()
		wg.Wait()
	}
	return poller, handler, item.ID, cleanup
}

func writeTestPlanFile(t *testing.T, root string, relPath string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nid: factory-request\ntitle: Factory Request\n---\n\n# Factory Request\n"
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
