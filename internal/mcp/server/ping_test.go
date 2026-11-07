package server

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
)

func TestPingNotificationsAndInvalidRequestsDoNotDispatch(t *testing.T) {
	handler := &testHandler{method: "ping", result: struct{}{}}
	r := router.New()
	r.Register(handler)
	input := strings.NewReader(
		`{"jsonrpc":"2.0","method":"ping"}` + "\n" +
			`{"jsonrpc":"2.0","method":"ping","params":{"_meta":false}}` + "\n" +
			`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"_meta":{"progressToken":{}}}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"ping"}` + "\n")
	var output bytes.Buffer
	if err := New(input, &output, r).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := `{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"invalid params"}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"result":{}}` + "\n"
	if handler.called != 1 || output.String() != want {
		t.Fatalf("handler calls=%d, stdout=%q", handler.called, output.String())
	}
}
