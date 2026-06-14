package factorydaemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/config"
	factorypkg "github.com/applauselab/bachkator/internal/factory"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/pkg/triggerprotocol"
)

type triggerPoller struct {
	service         Service
	factoryService  factorypkg.Service
	factory         string
	trigger         *config.FactoryProviderTrigger
	defaultWorkflow string
	session         *providerSession[*triggerprotocol.Client]
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
				session: newProviderSession(
					trigger.Name,
					trigger.Command,
					s.ConfigProject.Root,
					triggerprotocol.NewClient,
					newTriggerDial(trigger.Name, s.Factory.Name, trigger.Config),
				),
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

func newTriggerDial(
	name string,
	factory string,
	providerConfig map[string]string,
) func(context.Context, *triggerprotocol.Client) error {
	return func(ctx context.Context, client *triggerprotocol.Client) error {
		result, err := client.Handshake(ctx, triggerprotocol.HandshakeParams{
			Protocol: triggerprotocol.ProtocolVersion,
			Factory:  factory,
			Trigger:  name,
			Config:   providerConfig,
		})
		if err != nil {
			return err
		}
		if result.Protocol != triggerprotocol.ProtocolVersion {
			return fmt.Errorf(
				"trigger provider %q returned unsupported protocol %q",
				name,
				result.Protocol,
			)
		}
		if !hasCapability(result.Capabilities, triggerprotocol.CapabilityPoll) {
			return fmt.Errorf("trigger provider %q does not support poll", name)
		}
		return nil
	}
}

func (s Service) defaultWorkflow() string {
	if len(s.Factory.Workflows) == 1 && s.Factory.Workflows[0] != nil {
		return s.Factory.Workflows[0].Name
	}
	return ""
}

func (p *triggerPoller) run(ctx context.Context) {
	defer p.session.close()
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
	client, err := p.session.get(ctx)
	if err != nil {
		p.logf("trigger provider %q handshake failed: %v", p.trigger.Name, err)
		return
	}
	cursor, err := p.service.Backend.Factory.GetTriggerCursor(ctx, p.factory, p.trigger.Name)
	if err != nil {
		p.logf("trigger provider %q cursor read failed: %v", p.trigger.Name, err)
		p.session.invalidate()
		return
	}
	pollCtx, cancel := context.WithTimeout(ctx, providerCallTimeout)
	result, err := client.Poll(pollCtx, triggerprotocol.PollParams{
		Cursor: cursor.Cursor,
		Config: p.trigger.Config,
	})
	cancel()
	if err != nil {
		p.logf("trigger provider %q poll failed: %v", p.trigger.Name, err)
		p.recordErrorCursor(ctx, cursor.Cursor, err)
		p.session.invalidate()
		return
	}
	var sourceIDs []string
	if len(result.Items) > 0 {
		sourceIDs, err = p.processItems(ctx, result.Items)
		if err != nil {
			p.logf("trigger provider %q intake failed: %v", p.trigger.Name, err)
			_ = p.nack(ctx, client, result.Cursor, err)
			p.recordErrorCursor(ctx, cursor.Cursor, err)
			return
		}
	}
	if err := p.recordAckCursor(ctx, result.Cursor); err != nil {
		p.logf("trigger provider %q cursor record failed: %v", p.trigger.Name, err)
		_ = p.nack(ctx, client, result.Cursor, err)
		return
	}
	if len(sourceIDs) > 0 || result.Cursor != cursor.Cursor {
		if err := p.ack(ctx, client, result.Cursor, sourceIDs); err != nil {
			p.logf("trigger provider %q ack failed: %v", p.trigger.Name, err)
			p.session.invalidate()
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

func (p *triggerPoller) ack(
	ctx context.Context,
	client *triggerprotocol.Client,
	cursor string,
	sourceIDs []string,
) error {
	ackCtx, cancel := context.WithTimeout(ctx, providerCallTimeout)
	defer cancel()
	return client.Ack(ackCtx, triggerprotocol.AckParams{
		Cursor:    cursor,
		SourceIDs: sourceIDs,
	})
}

func (p *triggerPoller) nack(
	ctx context.Context,
	client *triggerprotocol.Client,
	cursor string,
	cause error,
) error {
	nackCtx, cancel := context.WithTimeout(ctx, providerCallTimeout)
	defer cancel()
	return client.Nack(nackCtx, triggerprotocol.NackParams{
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

func (p *triggerPoller) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(
		p.service.stderr(),
		"trigger "+p.trigger.Name+": "+format+"\n",
		args...,
	)
}

func hasCapability[C ~string](capabilities []C, required C) bool {
	for _, c := range capabilities {
		if c == required {
			return true
		}
	}
	return false
}
