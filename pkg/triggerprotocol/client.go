package triggerprotocol

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/applauselab/bachkator/pkg/jsonrpcstdio"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type Client struct {
	reader *bufio.Reader
	writer io.Writer
	nextID int
}

func NewClient(r io.Reader, w io.Writer) *Client {
	return &Client{
		reader: bufio.NewReader(r),
		writer: w,
	}
}

func (c *Client) Handshake(ctx context.Context, params HandshakeParams) (HandshakeResult, error) {
	var result HandshakeResult
	err := c.call(ctx, "trigger.handshake", params, &result)
	return result, err
}

func (c *Client) Poll(ctx context.Context, params PollParams) (PollResult, error) {
	var result PollResult
	err := c.call(ctx, "trigger.poll", params, &result)
	return result, err
}

func (c *Client) Ack(ctx context.Context, params AckParams) error {
	return c.call(ctx, "trigger.ack", params, nil)
}

func (c *Client) Nack(ctx context.Context, params NackParams) error {
	return c.call(ctx, "trigger.nack", params, nil)
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	_ = ctx
	c.nextID++
	rawParams, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("encode %s params: %w", method, err)
	}
	request, err := json.Marshal(Request{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  method,
		Params:  rawParams,
	})
	if err != nil {
		return fmt.Errorf("encode %s request: %w", method, err)
	}
	if err := jsonrpcstdio.WriteMessage(c.writer, request); err != nil {
		return fmt.Errorf("write %s message: %w", method, err)
	}
	payload, err := jsonrpcstdio.ReadMessage(c.reader)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("trigger provider closed stdout during %s", method)
		}
		return fmt.Errorf("read %s response: %w", method, err)
	}
	var response Response
	if err := json.Unmarshal(payload, &response); err != nil {
		return fmt.Errorf("decode %s response: %w", method, err)
	}
	if response.Error != nil {
		if response.Error.Data != nil {
			encoded, err := json.Marshal(response.Error.Data)
			if err == nil {
				var domainErr Error
				if err := json.Unmarshal(encoded, &domainErr); err == nil && domainErr.Code != "" {
					return fmt.Errorf("trigger %s: %w", method, domainErr)
				}
			}
		}
		return fmt.Errorf("trigger %s: %s", method, response.Error.Message)
	}
	if result != nil && len(response.Result) > 0 {
		if err := json.Unmarshal(response.Result, result); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
	}
	return nil
}
