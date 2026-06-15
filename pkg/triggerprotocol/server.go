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

type Handler interface {
	Handshake(context.Context, HandshakeParams) (HandshakeResult, error)
	Poll(context.Context, PollParams) (PollResult, error)
	Ack(context.Context, AckParams) error
	Nack(context.Context, NackParams) error
}

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
			if err := writeError(w, nil, NewError(ErrorInvalidRequest, err.Error())); err != nil {
				return err
			}
			continue
		}
		result, err := dispatch(ctx, request, handler)
		if err != nil {
			if err := writeError(w, request.ID, err); err != nil {
				return err
			}
			continue
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			if err := writeError(w, request.ID, NewError(ErrorInternal, err.Error())); err != nil {
				return err
			}
			continue
		}
		response := Response{JSONRPC: "2.0", ID: request.ID, Result: encoded}
		if err := writeResponse(w, response); err != nil {
			return err
		}
	}
}

func dispatch(
	ctx context.Context,
	request Request,
	handler Handler,
) (any, error) {
	switch request.Method {
	case "trigger.handshake":
		var params HandshakeParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, NewError(ErrorInvalidRequest, err.Error())
		}
		return handler.Handshake(ctx, params)
	case "trigger.poll":
		var params PollParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, NewError(ErrorInvalidRequest, err.Error())
		}
		return handler.Poll(ctx, params)
	case "trigger.ack":
		var params AckParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, NewError(ErrorInvalidRequest, err.Error())
		}
		return nil, handler.Ack(ctx, params)
	case "trigger.nack":
		var params NackParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, NewError(ErrorInvalidRequest, err.Error())
		}
		return nil, handler.Nack(ctx, params)
	default:
		return nil, NewError(ErrorInvalidRequest, "unknown method "+request.Method)
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
