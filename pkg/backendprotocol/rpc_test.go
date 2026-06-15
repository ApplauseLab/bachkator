package backendprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/applauselab/bachkator/pkg/jsonrpcstdio"
)

func TestServeWritesResult(t *testing.T) {
	request := Request{JSONRPC: "2.0", ID: float64(1), Method: "backend.initialize"}
	input := framedRequest(t, request)
	var output bytes.Buffer

	err := Serve(
		context.Background(),
		bytes.NewReader(input),
		&output,
		func(context.Context, Request) (any, error) {
			return InitializeResult{Protocol: ProtocolVersion, Provider: "test", Version: "v1"}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"provider":"test"`)) {
		t.Fatalf("output = %s", output.String())
	}
}

func framedRequest(t *testing.T, request Request) []byte {
	t.Helper()
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := jsonrpcstdio.WriteMessage(&buffer, encoded); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
