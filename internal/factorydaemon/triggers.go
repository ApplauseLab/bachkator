package factorydaemon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/config"
	factorypkg "github.com/applauselab/bachkator/internal/factory"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/pkg/triggerprotocol"
)

const triggerCallTimeout = 30 * time.Second

type triggerPoller struct {
	service         Service
	factoryService  factorypkg.Service
	factory         string
	trigger         *config.FactoryProviderTrigger
	defaultWorkflow string
	session         *triggerSession
}

type triggerSession struct {
	mu     sync.Mutex
	client *triggerprotocol.Client
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	cancel context.CancelFunc
	stderr *cappedBuffer
}

func (s Service) startProviderTriggers(ctx context.Context) <-chan error {
	errCh := make(chan error, 1)
	providers := s.Factory.ProviderTriggers()
	if len(providers) == 0 {
		close(errCh)
		return errCh
	}
	factoryService := factorypkg.Service{
		Root:  s.ConfigProject.Root,
		Queue: factorypkg.BackendQueue{Client: &s.Backend.Factory},
		NewID: s.NewID,
		Now:   s.Now,
	}
	go func() {
		defer close(errCh)
		var wg sync.WaitGroup
		for _, trigger := range providers {
			if trigger == nil {
				continue
			}
			poller := &triggerPoller{
				service:         s,
				factoryService:  factoryService,
				factory:         s.Factory.Name,
				trigger:         trigger,
				defaultWorkflow: s.defaultWorkflow(),
				session:         &triggerSession{},
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				poller.run(ctx)
			}()
		}
		wg.Wait()
	}()
	return errCh
}

func (s Service) defaultWorkflow() string {
	if len(s.Factory.Workflows) == 1 && s.Factory.Workflows[0] != nil {
		return s.Factory.Workflows[0].Name
	}
	return ""
}

func (p *triggerPoller) run(ctx context.Context) {
	defer p.closeSession()
	ticker := time.NewTicker(p.trigger.PollIntervalDuration())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		p.poll(ctx)
	}
}

func (p *triggerPoller) poll(ctx context.Context) {
	if err := p.ensureSession(ctx); err != nil {
		p.logf("trigger provider %q handshake failed: %v", p.trigger.Name, err)
		return
	}
	cursor, err := p.service.Backend.Factory.GetTriggerCursor(ctx, p.factory, p.trigger.Name)
	if err != nil {
		p.logf("trigger provider %q cursor read failed: %v", p.trigger.Name, err)
		p.invalidateSession()
		return
	}
	pollCtx, cancel := context.WithTimeout(ctx, triggerCallTimeout)
	result, err := p.session.client.Poll(pollCtx, triggerprotocol.PollParams{
		Cursor: cursor.Cursor,
		Config: p.trigger.Config,
	})
	cancel()
	if err != nil {
		p.logf("trigger provider %q poll failed: %v", p.trigger.Name, err)
		p.recordErrorCursor(ctx, cursor.Cursor, err)
		p.invalidateSession()
		return
	}
	var sourceIDs []string
	if len(result.Items) > 0 {
		sourceIDs, err = p.processItems(ctx, result.Items)
		if err != nil {
			p.logf("trigger provider %q intake failed: %v", p.trigger.Name, err)
			_ = p.nack(ctx, result.Cursor, err)
			p.recordErrorCursor(ctx, cursor.Cursor, err)
			return
		}
	}
	if err := p.recordAckCursor(ctx, result.Cursor); err != nil {
		p.logf("trigger provider %q cursor record failed: %v", p.trigger.Name, err)
		_ = p.nack(ctx, result.Cursor, err)
		return
	}
	if len(sourceIDs) > 0 || result.Cursor != cursor.Cursor {
		if err := p.ack(ctx, result.Cursor, sourceIDs); err != nil {
			p.logf("trigger provider %q ack failed: %v", p.trigger.Name, err)
			p.invalidateSession()
			return
		}
	}
}

func (p *triggerPoller) processItems(
	ctx context.Context,
	items []triggerprotocol.PollItem,
) ([]string, error) {
	sourceIDs := make([]string, 0, len(items))
	for _, item := range items {
		workflow, err := p.trigger.RouteWorkflow(item.Labels, p.defaultWorkflow)
		if err != nil {
			return nil, err
		}
		_, err = p.factoryService.ProviderIntake(ctx, factorypkg.ProviderIntakeOptions{
			Factory:        p.factory,
			Trigger:        p.trigger.Name,
			Workflow:       workflow,
			SourceType:     item.Source.Type,
			SourceID:       item.Source.ID,
			SourceURL:      item.Source.URL,
			SourceRevision: item.Source.Revision,
			Title:          item.Title,
			Body:           item.Body,
			Labels:         item.Labels,
			Priority:       model.Priority(item.Priority),
			Metadata:       item.Metadata,
			CreatedAt:      clock.UTC(p.service.Now),
		})
		if err != nil {
			return nil, err
		}
		sourceIDs = append(sourceIDs, item.Source.ID)
	}
	return sourceIDs, nil
}

func (p *triggerPoller) ack(ctx context.Context, cursor string, sourceIDs []string) error {
	ackCtx, cancel := context.WithTimeout(ctx, triggerCallTimeout)
	defer cancel()
	return p.session.client.Ack(ackCtx, triggerprotocol.AckParams{
		Cursor:    cursor,
		SourceIDs: sourceIDs,
	})
}

func (p *triggerPoller) nack(ctx context.Context, cursor string, cause error) error {
	nackCtx, cancel := context.WithTimeout(ctx, triggerCallTimeout)
	defer cancel()
	return p.session.client.Nack(nackCtx, triggerprotocol.NackParams{
		Cursor: cursor,
		Reason: cause.Error(),
	})
}

func (p *triggerPoller) recordAckCursor(ctx context.Context, cursor string) error {
	now := clock.UTC(p.service.Now)
	_, err := p.service.Backend.Factory.RecordTriggerCursor(ctx, backend.FactoryTriggerCursor{
		Factory:    p.factory,
		Trigger:    p.trigger.Name,
		Cursor:     cursor,
		LastPollAt: now,
		LastAckAt:  now,
		UpdatedAt:  now,
	})
	return err
}

func (p *triggerPoller) recordErrorCursor(ctx context.Context, cursor string, cause error) {
	now := clock.UTC(p.service.Now)
	_, _ = p.service.Backend.Factory.RecordTriggerCursor(ctx, backend.FactoryTriggerCursor{
		Factory:    p.factory,
		Trigger:    p.trigger.Name,
		Cursor:     cursor,
		LastPollAt: now,
		LastNackAt: now,
		LastError:  cause.Error(),
		UpdatedAt:  now,
	})
}

func (p *triggerPoller) ensureSession(ctx context.Context) error {
	p.session.mu.Lock()
	defer p.session.mu.Unlock()
	if p.session.client != nil {
		return nil
	}
	command, err := resolveTriggerCommand(p.trigger.Command)
	if err != nil {
		return err
	}
	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, command[0], command[1:]...)
	cmd.Dir = p.service.ConfigProject.Root
	cmd.Env = triggerEnvironment(p.trigger.Config)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		cancel()
		return err
	}
	stderrBuf := &cappedBuffer{limit: 64 * 1024}
	cmd.Stderr = stderrBuf
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		cancel()
		return err
	}
	client := triggerprotocol.NewClient(stdout, stdin)
	hsCtx, hsCancel := context.WithTimeout(ctx, triggerCallTimeout)
	result, err := client.Handshake(hsCtx, triggerprotocol.HandshakeParams{
		Protocol: triggerprotocol.ProtocolVersion,
		Factory:  p.factory,
		Trigger:  p.trigger.Name,
		Config:   p.trigger.Config,
	})
	hsCancel()
	if err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		cancel()
		return err
	}
	if result.Protocol != triggerprotocol.ProtocolVersion {
		_ = stdin.Close()
		_ = cmd.Wait()
		cancel()
		return fmt.Errorf(
			"trigger provider %q returned unsupported protocol %q",
			p.trigger.Name,
			result.Protocol,
		)
	}
	if !hasTriggerCapability(result.Capabilities, triggerprotocol.CapabilityPoll) {
		_ = stdin.Close()
		_ = cmd.Wait()
		cancel()
		return fmt.Errorf("trigger provider %q does not support poll", p.trigger.Name)
	}
	p.session.client = client
	p.session.cmd = cmd
	p.session.stdin = stdin
	p.session.cancel = cancel
	p.session.stderr = stderrBuf
	return nil
}

func (p *triggerPoller) invalidateSession() {
	p.session.mu.Lock()
	defer p.session.mu.Unlock()
	p.session.client = nil
}

func (p *triggerPoller) closeSession() {
	p.session.mu.Lock()
	defer p.session.mu.Unlock()
	if p.session.cancel != nil {
		p.session.cancel()
	}
	if p.session.stdin != nil {
		_ = p.session.stdin.Close()
	}
	if p.session.cmd != nil {
		_ = p.session.cmd.Wait()
	}
	p.session.client = nil
}

func (p *triggerPoller) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(p.service.stderr(), "trigger: "+format+"\n", args...)
}

func resolveTriggerCommand(command []string) ([]string, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("trigger provider command is empty")
	}
	if command[0] != "bach" {
		return command, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	resolved := append([]string{executable}, command[1:]...)
	return resolved, nil
}

func triggerEnvironment(config map[string]string) []string {
	env := []string{}
	seen := map[string]struct{}{}
	keys := []string{"PATH", "TMPDIR", "TEMP", "TMP"}
	if config != nil && config["token_env"] != "" {
		keys = append(keys, config["token_env"])
	}
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func hasTriggerCapability(
	capabilities []triggerprotocol.Capability,
	required triggerprotocol.Capability,
) bool {
	for _, c := range capabilities {
		if c == required {
			return true
		}
	}
	return false
}

type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
	mu     sync.Mutex
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
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
