package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/patriceckhart/zot/packages/agent/extproto"
)

func TestToolCancelCapabilityHandshake(t *testing.T) {
	for _, tc := range []struct {
		name string
		caps []string
		want bool
	}{
		{name: "omitted"},
		{name: "legacy", caps: []string{"tools", "panels"}},
		{name: "unknown", caps: []string{"tool_cancel_future", "TOOL_CANCEL"}},
		{name: "supported", caps: []string{"tools", "tool_cancel"}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeMockToolExtension(t, filepath.Join(root, "extensions"))
			path := filepath.Join(root, "extensions", "tool-mock", "run.sh")
			script, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hello, err := json.Marshal(extproto.HelloFromExt{Type: "hello", Name: "tool-mock", Version: "0.1", Capabilities: tc.caps})
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(script), "\n")
			lines[1] = "printf '%s\\n' '" + string(hello) + "'"
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o755); err != nil {
				t.Fatal(err)
			}
			m := New(root, "", "test", "", "", &stubHooks{})
			t.Cleanup(func() { m.Stop(time.Second) })
			if errs := m.Discover(context.Background()); len(errs) != 0 {
				t.Fatal(errs)
			}
			m.WaitForReady(time.Second)
			m.mu.RLock()
			ext := m.toolIndex["echo"]
			m.mu.RUnlock()
			if ext == nil {
				t.Fatal("tool was not registered")
			}
			if ext.toolCancel != tc.want {
				t.Fatalf("cancellation support = %v, want %v", ext.toolCancel, tc.want)
			}
		})
	}
}

func TestToolCancelCapabilityDelivery(t *testing.T) {
	for _, supported := range []bool{false, true} {
		name := "legacy"
		if supported {
			name = "supported"
		}
		for _, outcome := range []string{"cancel", "timeout", "complete"} {
			t.Run(name+"/"+outcome, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					m, ext, frames, replies := toolPeer(t)
					ext.toolCancel = supported
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					timeout := time.Duration(0)
					if outcome == "timeout" {
						timeout = time.Nanosecond
					}
					done := make(chan error, 1)
					go func() {
						_, err := m.InvokeTool(ctx, "ask", nil, timeout)
						done <- err
					}()
					call := toolCallFrame(t, frames)
					synctest.Wait()
					var want error
					switch outcome {
					case "cancel":
						want = context.Canceled
						cancel()
					case "timeout":
						want = context.DeadlineExceeded
					case "complete":
						sendToolFrame(t, replies, extproto.ToolResultFromExt{Type: "tool_result", ID: call.ID})
					}
					if err := awaitToolError(t, done); !errors.Is(err, want) {
						t.Fatalf("invocation error = %v, want %v", err, want)
					}
					if supported && outcome != "complete" {
						var frame extproto.ToolCancelFromHost
						if err := json.Unmarshal(nextLifecycleFrame(t, frames), &frame); err != nil {
							t.Fatal(err)
						}
						if frame.Type != "tool_cancel" || frame.ID != call.ID {
							t.Fatalf("unexpected cancellation frame: %+v", frame)
						}
					}
					synctest.Wait()
					select {
					case frame := <-frames:
						t.Fatalf("unexpected host frame: %s", frame)
					default:
					}
					ext.mu.Lock()
					pending := len(ext.pendingTool)
					ext.mu.Unlock()
					if pending != 0 {
						t.Fatal("pending invocation leaked")
					}
					// Abandoned results remain harmless without cancellation support.
					sendToolFrame(t, replies, extproto.ToolResultFromExt{Type: "tool_result", ID: call.ID})
					go func() {
						_, err := m.InvokeTool(context.Background(), "ask", nil, 0)
						done <- err
					}()
					next := toolCallFrame(t, frames)
					sendToolFrame(t, replies, extproto.ToolResultFromExt{Type: "tool_result", ID: next.ID})
					if err := awaitToolError(t, done); err != nil {
						t.Fatal(err)
					}
				})
			})
		}
	}
}
