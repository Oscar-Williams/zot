package ext

import (
	"context"
	"encoding/json"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/patriceckhart/zot/packages/agent/extproto"
)

func TestInteractiveToolLifecycle(t *testing.T) {
	for _, end := range []string{"cancel", "shutdown", "eof", "complete"} {
		t.Run(end, func(t *testing.T) {
			h := newHarness("interactive")
			started := make(chan context.Context, 2)
			finished := make(chan struct{}, 2)
			answer := make(chan struct{})
			h.ext.InteractiveTool("ask", "Ask the user", json.RawMessage(`{}`), func(ctx context.Context, _ json.RawMessage) ToolResult {
				started <- ctx
				defer func() { finished <- struct{}{} }()
				select {
				case <-ctx.Done():
					return TextErrorResult("cancelled")
				case <-answer:
					return TextResult("answer")
				}
			})
			done := make(chan error, 1)
			go func() { done <- h.ext.Run() }()
			t.Cleanup(func() {
				h.hostW.Close()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(time.Second):
					t.Error("SDK did not exit")
				}
				h.ext.out.(io.Closer).Close()
				h.ext.in.(io.Closer).Close()
			})
			var hello extproto.HelloFromExt
			if err := json.Unmarshal(h.next(t).raw, &hello); err != nil {
				t.Fatal(err)
			}
			if hello.Type != "hello" || !slices.Contains(hello.Capabilities, "tool_cancel") {
				t.Fatalf("SDK did not advertise cancellation support: %+v", hello)
			}
			h.sendToExt(t, extproto.HelloAckFromHost{Type: "hello_ack"})
			var registration extproto.RegisterToolFromExt
			json.Unmarshal(h.drainUntil(t, "register_tool").raw, &registration)
			if !registration.Interactive || registration.Name != "ask" {
				t.Fatalf("registration: %+v", registration)
			}
			h.drainUntil(t, "ready")
			h.sendToExt(t, extproto.ToolCallFromHost{Type: "tool_call", ID: "one", Name: "ask", Args: json.RawMessage(`{}`)})
			// Send cancellation without waiting for the goroutine to start.
			if end == "cancel" {
				h.sendToExt(t, extproto.ToolCancelFromHost{Type: "tool_cancel", ID: "one"})
			}
			var ctx context.Context
			select {
			case ctx = <-started:
			case <-time.After(time.Second):
				t.Fatal("handler did not start")
			}
			switch end {
			case "shutdown":
				h.sendToExt(t, extproto.ShutdownFromHost{Type: "shutdown"})
				h.drainUntil(t, "shutdown_ack")
			case "eof":
				h.hostW.Close()
			case "complete":
				close(answer)
				var result extproto.ToolResultFromExt
				json.Unmarshal(h.drainUntil(t, "tool_result").raw, &result)
				if result.ID != "one" || len(result.Content) != 1 || result.Content[0].Text != "answer" {
					t.Fatalf("result: %+v", result)
				}
			}
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("handler did not finish")
			}
			if end != "complete" && ctx.Err() != context.Canceled {
				t.Fatal("handler context was not cancelled")
			}
			if end == "cancel" {
				// Unknown and duplicate cancellation must not affect another call.
				h.sendToExt(t, extproto.ToolCancelFromHost{Type: "tool_cancel", ID: "one"})
				h.sendToExt(t, extproto.ToolCancelFromHost{Type: "tool_cancel", ID: "unknown"})
				h.sendToExt(t, extproto.ToolCallFromHost{Type: "tool_call", ID: "two", Name: "ask", Args: json.RawMessage(`{}`)})
				select {
				case ctx = <-started:
				case <-time.After(time.Second):
					t.Fatal("second handler did not start")
				}
				if ctx.Err() != nil {
					t.Fatal("second call was cancelled")
				}
				close(answer)
				var result extproto.ToolResultFromExt
				json.Unmarshal(h.drainUntil(t, "tool_result").raw, &result)
				if result.ID != "two" {
					t.Fatalf("unexpected result: %+v", result)
				}
			}
		})
	}
}
