package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/applauselab/bachkator/pkg/approvalprotocol"
)

type fixtureProvider struct {
	itemsPath string
}

func main() {
	if err := approvalprotocol.Serve(
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
	params approvalprotocol.HandshakeParams,
) (approvalprotocol.HandshakeResult, error) {
	if params.Protocol != approvalprotocol.ProtocolVersion {
		return approvalprotocol.HandshakeResult{}, approvalprotocol.NewError(
			approvalprotocol.ErrorUnsupportedProtocol,
			"unsupported protocol "+params.Protocol,
		)
	}
	p.itemsPath = params.Config["items_path"]
	return approvalprotocol.HandshakeResult{
		Protocol:     approvalprotocol.ProtocolVersion,
		Provider:     "bach-approval-fixture",
		Version:      "v1",
		Capabilities: []approvalprotocol.Capability{approvalprotocol.CapabilityPoll},
	}, nil
}

func (p *fixtureProvider) Poll(
	_ context.Context,
	_ approvalprotocol.PollParams,
) (approvalprotocol.PollResult, error) {
	records, err := p.readApprovals()
	if err != nil {
		return approvalprotocol.PollResult{}, approvalprotocol.NewError(
			approvalprotocol.ErrorInternal,
			"read approvals: "+err.Error(),
		)
	}
	return approvalprotocol.PollResult{Approvals: records}, nil
}

func (p *fixtureProvider) readApprovals() ([]approvalprotocol.ApprovalRecord, error) {
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
	var records []approvalprotocol.ApprovalRecord
	if len(data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}
