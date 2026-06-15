package backendprotocol

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

type Handler func(context.Context, Request) (any, error)

func Serve(ctx context.Context, r io.Reader, w io.Writer, handler Handler) error {
	reader := bufio.NewReader(r)
	for {
		payload, err := jsonrpcstdio.ReadMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		var request Request
		if err := json.Unmarshal(payload, &request); err != nil {
			return writeError(w, nil, NewError(ErrorInvalidRequest, err.Error()))
		}
		result, err := handler(ctx, request)
		if err != nil {
			if err := writeError(w, request.ID, err); err != nil {
				return err
			}
			continue
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return writeError(w, request.ID, NewError(ErrorInternal, err.Error()))
		}
		response := Response{JSONRPC: "2.0", ID: request.ID, Result: encoded}
		if err := writeResponse(w, response); err != nil {
			return err
		}
	}
}

func writeError(w io.Writer, id any, err error) error {
	domainErr := Error{Code: ErrorInternal, Message: err.Error()}
	var candidate Error
	if errors.As(err, &candidate) {
		domainErr = candidate
	}
	return writeResponse(w, Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ResponseError{
			Code:    -32000,
			Message: domainErr.Message,
			Data:    domainErr,
		},
	})
}

func writeResponse(w io.Writer, response Response) error {
	encoded, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("encode json-rpc response: %w", err)
	}
	return jsonrpcstdio.WriteMessage(w, encoded)
}
