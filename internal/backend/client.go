package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/backend/sqlite"
	"github.com/applauselab/bachkator/internal/model"
	statestore "github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
	"github.com/applauselab/bachkator/pkg/jsonrpcstdio"
)

const testProviderHelperEnv = "BACH_BACKEND_PROVIDER_TEST_HELPER"

const providerCallTimeout = 30 * time.Second

func init() {
	if os.Getenv(testProviderHelperEnv) != "sqlite" {
		return
	}
	executable, err := os.Executable()
	if err != nil || !strings.HasSuffix(executable, ".test") {
		return
	}
	if err := sqlite.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

type State = statestore.State
type StateRecord = statestore.Record
type RunRecord = statestore.RunRecord
type TargetRunRecord = statestore.TargetRunRecord
type ArtifactRecord = statestore.ArtifactRecord
type QualityReport = statestore.QualityReport
type QualityGateResult = statestore.QualityGateResult
type NormalizedFinding = statestore.NormalizedFinding
type RunQuery = statestore.RunQuery

type Client struct {
	root     string
	path     string
	backend  model.Backend
	provider bool
	Runs     RunsClient
	Evidence EvidenceClient
	Quality  QualityClient
	Findings FindingsClient
	Factory  FactoryQueueClient
	Plans    PlanLedgerClient
}

type RunsClient struct{ client *Client }
type EvidenceClient struct{ client *Client }
type QualityClient struct{ client *Client }
type FindingsClient struct{ client *Client }
type FactoryQueueClient struct{ client *Client }
type PlanLedgerClient struct{ client *Client }

func NewClient(path string) *Client {
	client := &Client{
		path: path,
	}
	client.Runs = RunsClient{client: client}
	client.Evidence = EvidenceClient{client: client}
	client.Quality = QualityClient{client: client}
	client.Findings = FindingsClient{client: client}
	client.Factory = FactoryQueueClient{client: client}
	client.Plans = PlanLedgerClient{client: client}
	return client
}

func NewProjectClient(root string, path string, backend model.Backend) *Client {
	client := NewClient(path)
	client.root = root
	client.backend = backend
	client.provider = backend.Type == "stdio" && len(backend.Command) > 0
	client.Runs = RunsClient{client: client}
	client.Evidence = EvidenceClient{client: client}
	client.Quality = QualityClient{client: client}
	client.Findings = FindingsClient{client: client}
	client.Factory = FactoryQueueClient{client: client}
	client.Plans = PlanLedgerClient{client: client}
	return client
}

func (c Client) Load(ctx context.Context) (*State, error) {
	_ = ctx
	store, err := statestore.NewStore(c.path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return store.Load()
}

func (c Client) LoadReadOnly(ctx context.Context) (*State, error) {
	_ = ctx
	store, err := statestore.OpenReadOnlyStore(c.path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return store.Load()
}

func withStore[T any](
	ctx context.Context,
	path string,
	fn func(*statestore.Store) (T, error),
) (T, error) {
	_ = ctx
	var zero T
	store, err := statestore.NewStore(path)
	if err != nil {
		return zero, err
	}
	defer func() { _ = store.Close() }()
	return fn(store)
}

func (c QualityClient) RecordReports(
	ctx context.Context,
	reports []QualityReport,
	gates []QualityGateResult,
) error {
	if !c.client.provider {
		_, err := withStore(ctx, c.client.path, func(store *statestore.Store) (struct{}, error) {
			return struct{}{}, store.SaveQualityReports(reports, gates)
		})
		return err
	}
	return c.client.callProvider(
		ctx,
		"quality.recordReports",
		qualityBatchToProtocol(reports, gates),
	)
}

func (c FindingsClient) RecordObservation(
	ctx context.Context,
	finding backendprotocol.FindingObservation,
) error {
	if c.client.provider {
		return c.client.callProvider(ctx, "findings.recordObservation", finding)
	}
	var location *statestore.FindingLocation
	if finding.Location != nil {
		location = &statestore.FindingLocation{
			Path:        finding.Location.Path,
			StartLine:   finding.Location.StartLine,
			StartColumn: finding.Location.StartColumn,
			EndLine:     finding.Location.EndLine,
			EndColumn:   finding.Location.EndColumn,
		}
	}
	_, err := withStore(ctx, c.client.path, func(store *statestore.Store) (struct{}, error) {
		return struct{}{}, store.RecordFindingObservation(statestore.NormalizedFinding{
			ID:                   finding.ID,
			Fingerprint:          finding.Fingerprint,
			SourceType:           finding.SourceType,
			SourceID:             finding.SourceID,
			Severity:             string(finding.Severity),
			Category:             finding.Category,
			Message:              finding.Message,
			Location:             location,
			SuggestedFingerprint: finding.SuggestedFingerprint,
			ObservedAt:           finding.ObservedAt,
			Metadata:             finding.Metadata,
		})
	})
	return err
}

func (c Client) callProvider(ctx context.Context, method string, params any) error {
	callCtx, cancel := context.WithTimeout(ctx, providerCallTimeout)
	defer cancel()
	return c.withProviderSession(callCtx, method, func(session *providerSession) error {
		return session.call(method, params, nil)
	})
}

func (c Client) withProviderSession(
	ctx context.Context,
	method string,
	fn func(*providerSession) error,
) error {
	session, cleanup, err := c.openProviderSession(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	initResult := backendprotocol.InitializeResult{}
	if err := session.call("backend.initialize", backendprotocol.InitializeParams{
		Protocol:    backendprotocol.ProtocolVersion,
		ProjectName: filepath.Base(c.root),
		ProjectRoot: c.root,
		Config:      c.backend.Config,
	}, &initResult); err != nil {
		return err
	}
	if initResult.Protocol != backendprotocol.ProtocolVersion {
		return fmt.Errorf("backend provider returned protocol %q", initResult.Protocol)
	}
	if required := requiredCapability(
		method,
	); required != "" &&
		!hasCapability(initResult.Capabilities, required) {
		return backendprotocol.NewError(
			backendprotocol.ErrorUnsupportedCapability,
			fmt.Sprintf("backend provider does not support capability %q", required),
		)
	}
	if err := fn(session); err != nil {
		return err
	}
	return session.call("backend.shutdown", map[string]bool{}, nil)
}

func requiredCapability(method string) backendprotocol.Capability {
	switch {
	case strings.HasPrefix(method, "runs."):
		return backendprotocol.CapabilityRuns
	case strings.HasPrefix(method, "evidence."):
		return backendprotocol.CapabilityEvidenceRefs
	case strings.HasPrefix(method, "quality."):
		return backendprotocol.CapabilityQualityReports
	case strings.HasPrefix(method, "findings."):
		return backendprotocol.CapabilityFindings
	case strings.HasPrefix(method, "factory."):
		return backendprotocol.CapabilityFactoryQueue
	case strings.HasPrefix(method, "plans."):
		return backendprotocol.CapabilityPlanLedger
	default:
		return ""
	}
}

func hasCapability(
	capabilities []backendprotocol.Capability,
	required backendprotocol.Capability,
) bool {
	for _, capability := range capabilities {
		if capability == required {
			return true
		}
	}
	return false
}

type providerSession struct {
	reader *bufio.Reader
	writer io.Writer
	nextID int
}

func (s *providerSession) call(method string, params any, result any) error {
	s.nextID++
	rawParams, err := json.Marshal(params)
	if err != nil {
		return err
	}
	request, err := json.Marshal(backendprotocol.Request{
		JSONRPC: "2.0",
		ID:      s.nextID,
		Method:  method,
		Params:  rawParams,
	})
	if err != nil {
		return err
	}
	if err := jsonrpcstdio.WriteMessage(s.writer, request); err != nil {
		return err
	}
	payload, err := jsonrpcstdio.ReadMessage(s.reader)
	if err != nil {
		return err
	}
	var response backendprotocol.Response
	if err := json.Unmarshal(payload, &response); err != nil {
		return err
	}
	if response.Error != nil {
		if response.Error.Data != nil {
			encoded, err := json.Marshal(response.Error.Data)
			if err == nil {
				var domainErr backendprotocol.Error
				if err := json.Unmarshal(encoded, &domainErr); err == nil && domainErr.Code != "" {
					return fmt.Errorf("backend %s: %w", method, domainErr)
				}
			}
		}
		return fmt.Errorf("backend %s: %s", method, response.Error.Message)
	}
	if result != nil && len(response.Result) > 0 {
		if err := json.Unmarshal(response.Result, result); err != nil {
			return err
		}
	}
	return nil
}

func (c Client) openProviderSession(ctx context.Context) (*providerSession, func(), error) {
	return c.openProcessProvider(ctx)
}

func (c Client) openProcessProvider(ctx context.Context) (*providerSession, func(), error) {
	command := append([]string(nil), c.backend.Command...)
	if len(command) == 0 {
		return nil, nil, fmt.Errorf("backend command is empty")
	}
	env := providerEnvironment(false)
	if command[0] == "bach" {
		executable, err := os.Executable()
		if err != nil {
			return nil, nil, err
		}
		if strings.HasSuffix(executable, ".test") {
			command = []string{executable}
			env = providerEnvironment(true)
		} else {
			command[0] = executable
		}
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = c.root
	cmd.Env = env
	stderr := &cappedBuffer{limit: 64 * 1024}
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}
	return &providerSession{reader: bufio.NewReader(stdout), writer: stdin}, cleanup, nil
}

func providerEnvironment(testHelper bool) []string {
	env := []string{}
	for _, key := range []string{"PATH", "TMPDIR", "TEMP", "TMP"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	if testHelper {
		env = append(env, testProviderHelperEnv+"=sqlite")
	}
	return env
}

type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	available := b.limit - b.buffer.Len()
	if available > 0 {
		if len(data) > available {
			_, _ = b.buffer.Write(data[:available])
		} else {
			_, _ = b.buffer.Write(data)
		}
	}
	return len(data), nil
}

func qualityBatchToProtocol(
	reports []QualityReport,
	gates []QualityGateResult,
) backendprotocol.QualityReportBatch {
	batch := backendprotocol.QualityReportBatch{
		Reports: make([]backendprotocol.QualityReport, 0, len(reports)),
		Gates:   make([]backendprotocol.QualityGateResult, 0, len(gates)),
	}
	for _, report := range reports {
		batch.Reports = append(batch.Reports, qualityReportToProtocol(report))
	}
	for _, gate := range gates {
		batch.Gates = append(batch.Gates, backendprotocol.QualityGateResult{
			RunID:     gate.RunID,
			Target:    gate.Target,
			Metric:    gate.Metric,
			Op:        gate.Op,
			Threshold: gate.Threshold,
			Actual:    gate.Actual,
			Status:    gate.Status,
			Message:   gate.Message,
			CreatedAt: gate.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return batch
}

func qualityReportToProtocol(report QualityReport) backendprotocol.QualityReport {
	result := backendprotocol.QualityReport{
		RunID:     report.RunID,
		Target:    report.Target,
		Kind:      report.Kind,
		Format:    report.Format,
		Path:      report.Path,
		Status:    report.Status,
		Message:   report.Message,
		CreatedAt: report.CreatedAt.UTC().Format(time.RFC3339Nano),
		Metrics:   make([]backendprotocol.QualityMetric, 0, len(report.Metrics)),
		Findings:  make([]backendprotocol.QualityFinding, 0, len(report.Findings)),
	}
	for _, metric := range report.Metrics {
		result.Metrics = append(result.Metrics, backendprotocol.QualityMetric{
			Name:  metric.Name,
			Scope: metric.Scope,
			Value: metric.Value,
			Unit:  metric.Unit,
		})
	}
	for _, finding := range report.Findings {
		result.Findings = append(result.Findings, backendprotocol.QualityFinding{
			Kind:       finding.Kind,
			File:       finding.File,
			Line:       finding.Line,
			Severity:   finding.Severity,
			Rule:       finding.Rule,
			Message:    finding.Message,
			DurationMS: finding.DurationMS,
		})
	}
	return result
}
