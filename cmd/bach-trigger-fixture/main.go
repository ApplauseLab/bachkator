package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/applauselab/bachkator/pkg/triggerprotocol"
)

type fixtureProvider struct {
	itemsPath  string
	cursorPath string
}

func main() {
	if err := triggerprotocol.Serve(
		context.Background(),
		os.Stdin,
		os.Stdout,
		&fixtureProvider{},
	); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (p *fixtureProvider) Handshake(
	_ context.Context,
	params triggerprotocol.HandshakeParams,
) (triggerprotocol.HandshakeResult, error) {
	if params.Protocol != triggerprotocol.ProtocolVersion {
		return triggerprotocol.HandshakeResult{}, triggerprotocol.NewError(
			triggerprotocol.ErrorUnsupportedProtocol,
			"unsupported protocol "+params.Protocol,
		)
	}
	p.itemsPath = params.Config["items_path"]
	p.cursorPath = params.Config["cursor_path"]
	return triggerprotocol.HandshakeResult{
		Protocol: triggerprotocol.ProtocolVersion,
		Provider: "bach-trigger-fixture",
		Version:  "v1",
		Capabilities: []triggerprotocol.Capability{
			triggerprotocol.CapabilityPoll,
			triggerprotocol.CapabilityAck,
			triggerprotocol.CapabilityNack,
		},
	}, nil
}

func (p *fixtureProvider) Poll(
	_ context.Context,
	params triggerprotocol.PollParams,
) (triggerprotocol.PollResult, error) {
	_ = params
	items, err := p.readItems()
	if err != nil {
		return triggerprotocol.PollResult{}, triggerprotocol.NewError(
			triggerprotocol.ErrorInternal,
			"read items: "+err.Error(),
		)
	}
	cursor := "empty"
	if len(items) > 0 {
		cursor = "batch"
	}
	return triggerprotocol.PollResult{Cursor: cursor, Items: items}, nil
}

func (p *fixtureProvider) Ack(_ context.Context, params triggerprotocol.AckParams) error {
	if params.Cursor == "batch" {
		if err := p.writeFile(p.itemsPath, []byte("[]")); err != nil {
			return triggerprotocol.NewError(
				triggerprotocol.ErrorInternal,
				"clear items: "+err.Error(),
			)
		}
	}
	if p.cursorPath != "" {
		if err := p.writeFile(p.cursorPath, []byte(params.Cursor)); err != nil {
			return triggerprotocol.NewError(
				triggerprotocol.ErrorInternal,
				"write cursor: "+err.Error(),
			)
		}
	}
	return nil
}

func (p *fixtureProvider) Nack(_ context.Context, params triggerprotocol.NackParams) error {
	if p.cursorPath != "" {
		content := "nack: " + params.Reason
		if err := p.writeFile(p.cursorPath, []byte(content)); err != nil {
			return triggerprotocol.NewError(
				triggerprotocol.ErrorInternal,
				"write cursor: "+err.Error(),
			)
		}
	}
	return nil
}

func (p *fixtureProvider) readItems() ([]triggerprotocol.PollItem, error) {
	if p.itemsPath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(p.itemsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var items []triggerprotocol.PollItem
	if len(data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (p *fixtureProvider) writeFile(path string, data []byte) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(getDir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func getDir(path string) string {
	if idx := len(path) - 1; idx >= 0 {
		for i := idx; i >= 0; i-- {
			if path[i] == os.PathSeparator {
				return path[:i]
			}
		}
	}
	return "."
}
