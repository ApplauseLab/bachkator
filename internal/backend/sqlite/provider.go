package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/evidence"
	"github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

type Provider struct {
	initialized bool
	projectRoot string
	storePath   string
	Now         clock.NowFunc
}

func Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	provider := &Provider{}
	return backendprotocol.Serve(ctx, stdin, stdout, provider.handle)
}

func (p *Provider) handle(_ context.Context, request backendprotocol.Request) (any, error) {
	switch request.Method {
	case "backend.initialize":
		return p.initialize(request.Params)
	case "backend.shutdown":
		p.initialized = false
		return map[string]bool{"ok": true}, nil
	case "runs.create":
		return p.createRun(request.Params)
	case "runs.finish":
		return p.finishRun(request.Params)
	case "runs.get":
		return p.getRun(request.Params)
	case "runs.list":
		return p.listRuns(request.Params)
	case "runs.startTarget":
		return p.writeTargetRun(request.Params)
	case "runs.finishTarget":
		return p.writeTargetRun(request.Params)
	case "evidence.recordRef":
		return p.recordEvidenceRef(request.Params)
	case "evidence.listRefs":
		return p.listEvidenceRefs(request.Params)
	case "quality.recordReport":
		return p.recordQualityReport(request.Params)
	case "quality.recordReports":
		return p.recordQualityReports(request.Params)
	case "findings.recordObservation":
		return p.recordFindingObservation(request.Params)
	case "findings.get":
		return p.getFinding(request.Params)
	case "findings.listCurrent":
		return p.listCurrentFindings(request.Params)
	case "findings.listEvents":
		return p.listFindingEvents(request.Params)
	case "factory.enqueueWorkItem":
		return p.enqueueFactoryWorkItem(request.Params)
	case "factory.getWorkItem":
		return p.getFactoryWorkItem(request.Params)
	case "factory.listWorkItems":
		return p.listFactoryWorkItems(request.Params)
	case "factory.cancelWorkItem":
		return p.cancelFactoryWorkItem(request.Params)
	case "factory.acquireDaemonLease":
		return p.acquireFactoryDaemonLease(request.Params)
	case "factory.renewDaemonLease":
		return p.renewFactoryDaemonLease(request.Params)
	case "factory.releaseDaemonLease":
		return p.releaseFactoryDaemonLease(request.Params)
	case "factory.claimWorkItem":
		return p.claimFactoryWorkItem(request.Params)
	case "factory.updateWorkItemPhase":
		return p.updateFactoryWorkItemPhase(request.Params)
	case "factory.updatePendingWorkItem":
		return p.updatePendingFactoryWorkItem(request.Params)
	case "factory.completeWorkItem":
		return p.completeFactoryWorkItem(request.Params)
	case "factory.failWorkItem":
		return p.failFactoryWorkItem(request.Params)
	case "factory.getDaemonStatus":
		return p.getFactoryDaemonStatus(request.Params)
	case "factory.recordApproval":
		return p.recordFactoryApproval(request.Params)
	case "factory.listApprovals":
		return p.listFactoryApprovals(request.Params)
	case "factory.getTriggerCursor":
		return p.getFactoryTriggerCursor(request.Params)
	case "factory.recordTriggerCursor":
		return p.recordFactoryTriggerCursor(request.Params)
	case "plans.recordLedger":
		return p.recordPlanLedger(request.Params)
	case "plans.getLedger":
		return p.getPlanLedger(request.Params)
	default:
		if !p.initialized {
			return nil, backendprotocol.NewError(
				backendprotocol.ErrorNotInitialized,
				"backend provider is not initialized",
			)
		}
		return nil, backendprotocol.NewError(
			backendprotocol.ErrorUnsupportedCapability,
			fmt.Sprintf(
				"method %q is not implemented by the sqlite backend provider yet",
				request.Method,
			),
		)
	}
}

func (p *Provider) requireInitialized() error {
	if p.initialized {
		return nil
	}
	return backendprotocol.NewError(
		backendprotocol.ErrorNotInitialized,
		"backend provider is not initialized",
	)
}

func (p *Provider) now() time.Time {
	return clock.UTC(p.Now)
}

func (p *Provider) initialize(raw json.RawMessage) (backendprotocol.InitializeResult, error) {
	var params backendprotocol.InitializeParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.InitializeResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	if params.Protocol != backendprotocol.ProtocolVersion {
		return backendprotocol.InitializeResult{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"unsupported backend protocol",
		)
	}
	path := params.Config["path"]
	if path == "" {
		path = ".bach/state.db"
	}
	resolved, err := resolveDBPath(params.ProjectRoot, path)
	if err != nil {
		return backendprotocol.InitializeResult{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			err.Error(),
		)
	}
	var store *state.Store
	if err := withMigrationLock(resolved, func() (err error) {
		store, err = state.NewStore(resolved)
		if err != nil {
			return err
		}
		if _, err := store.Load(); err != nil {
			_ = store.Close()
			return err
		}
		return nil
	}); err != nil {
		return backendprotocol.InitializeResult{}, fmt.Errorf("initialize sqlite backend: %w", err)
	}
	p.initialized = true
	p.projectRoot = params.ProjectRoot
	p.storePath = resolved
	return backendprotocol.InitializeResult{
		Protocol: backendprotocol.ProtocolVersion,
		Provider: "bach-sqlite",
		Version:  "v1",
		Capabilities: []backendprotocol.Capability{
			backendprotocol.CapabilityRuns,
			backendprotocol.CapabilityEvidenceRefs,
			backendprotocol.CapabilityQualityReports,
			backendprotocol.CapabilityFindings,
			backendprotocol.CapabilityFactoryQueue,
			backendprotocol.CapabilityPlanLedger,
			backendprotocol.CapabilityApprovals,
		},
	}, nil
}

func resolveDBPath(root string, path string) (string, error) {
	return evidence.ResolveProjectStatePath(root, path)
}
