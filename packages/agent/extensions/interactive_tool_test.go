package extensions

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/patriceckhart/zot/packages/agent/extproto"
)

type interactiveHooks struct{ stubHooks }

func (*interactiveHooks) SupportsInteractiveTools() bool { return true }

func toolPeer(t *testing.T) (*Manager, *Extension, <-chan []byte, *io.PipeWriter) {
	t.Helper()
	m := New(t.TempDir(), "", "test", "", "", &interactiveHooks{})
	frames, ext := lifecyclePeer(t, m)
	ext.pendingTool = map[string]chan extproto.ToolResultFromExt{}
	ext.toolCancel = true
	ext.readyCh = make(chan struct{})
	log, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	ext.logFile = log
	r, w := io.Pipe()
	done := make(chan struct{})
	go func() {
		m.readLoop(ext, bufio.NewScanner(r))
		close(done)
	}()
	t.Cleanup(func() {
		w.Close()
		<-done
		r.Close()
		log.Close()
	})
	m.toolIndex["ask"] = ext
	return m, ext, frames, w
}

func sendToolFrame(t *testing.T, w io.Writer, frame any) {
	t.Helper()
	b, err := extproto.Encode(frame)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
}

func toolCallFrame(t *testing.T, frames <-chan []byte) extproto.ToolCallFromHost {
	t.Helper()
	var call extproto.ToolCallFromHost
	if err := json.Unmarshal(nextLifecycleFrame(t, frames), &call); err != nil {
		t.Fatal(err)
	}
	if call.Type != "tool_call" {
		t.Fatalf("expected tool_call, got %+v", call)
	}
	return call
}

func awaitToolError(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("invocation did not finish")
		return nil
	}
}

func TestInteractiveToolRegistrationAndCompletion(t *testing.T) {
	m, _, frames, replies := toolPeer(t)
	sendToolFrame(t, replies, extproto.RegisterToolFromExt{Type: "register_tool", Name: "ask", Schema: json.RawMessage(`{}`), Interactive: true})
	sendToolFrame(t, replies, extproto.ReadyFromExt{Type: "ready"})
	m.WaitForReady(time.Second)
	infos := m.Tools()
	if len(infos) != 1 || !infos[0].Interactive {
		t.Fatalf("registration: %+v", infos)
	}
	tool := NewTool(m, infos[0]).(*extensionTool)
	if tool.timeout != 0 {
		t.Fatal("interactive timeout is enabled")
	}
	if normal := NewTool(m, ToolInfo{}).(*extensionTool); normal.timeout != 60*time.Second {
		t.Fatal("normal timeout changed")
	}
	done := make(chan error, 1)
	go func() {
		result, err := tool.Execute(context.Background(), nil, nil)
		if err == nil && result.IsError {
			err = errors.New("interactive completion failed")
		}
		done <- err
	}()
	call := toolCallFrame(t, frames)
	sendToolFrame(t, replies, extproto.ToolResultFromExt{Type: "tool_result", ID: call.ID})
	if err := awaitToolError(t, done); err != nil {
		t.Fatal(err)
	}
}

func TestInteractiveToolTermination(t *testing.T) {
	for _, reason := range []string{"cancel", "timeout", "disconnect", "shutdown"} {
		t.Run(reason, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				m, ext, frames, replies := toolPeer(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				timeout := time.Duration(0)
				if reason == "timeout" {
					timeout = time.Nanosecond
				}
				done := make(chan error, 1)
				go func() {
					_, err := m.InvokeTool(ctx, "ask", nil, timeout)
					done <- err
				}()
				call := toolCallFrame(t, frames)
				// Ensure request delivery has completed before cancelling the reply wait.
				synctest.Wait()
				switch reason {
				case "cancel":
					cancel()
				case "disconnect":
					replies.Close()
				case "shutdown":
					m.Stop(time.Second)
				}
				err := awaitToolError(t, done)
				if err == nil {
					t.Fatal("expected terminal error")
				}
				if reason == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				if reason == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal(err)
				}
				ext.mu.Lock()
				pending := len(ext.pendingTool)
				ext.mu.Unlock()
				if pending != 0 {
					t.Fatal("pending call leaked")
				}
				if reason == "cancel" || reason == "timeout" {
					var frame extproto.ToolCancelFromHost
					json.Unmarshal(nextLifecycleFrame(t, frames), &frame)
					if frame.Type != "tool_cancel" || frame.ID != call.ID {
						t.Fatalf("cancel: %+v", frame)
					}
					// A late result must not block delivery of the next result.
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
				}
			})
		})
	}
}

func TestInteractiveToolConcurrentReplies(t *testing.T) {
	m, _, frames, replies := toolPeer(t)
	const count = 32
	done := make(chan error, count)
	for n := 0; n < count; n++ {
		go func() {
			_, err := m.InvokeTool(context.Background(), "ask", nil, 0)
			done <- err
		}()
	}
	ids := make([]string, count)
	seen := map[string]bool{}
	for n := range ids {
		ids[n] = toolCallFrame(t, frames).ID
		if seen[ids[n]] {
			t.Fatal("duplicate invocation ID")
		}
		seen[ids[n]] = true
	}
	for n := count - 1; n >= 0; n-- {
		sendToolFrame(t, replies, extproto.ToolResultFromExt{Type: "tool_result", ID: ids[n]})
	}
	for range ids {
		if err := awaitToolError(t, done); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInteractiveToolWaitsBeyondDefaultTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, ext, frames, replies := toolPeer(t)
		done := make(chan error, 1)
		go func() {
			_, err := m.InvokeTool(context.Background(), "ask", nil, 0)
			done <- err
		}()
		call := toolCallFrame(t, frames)
		synctest.Wait()
		// Advance the fake clock, not wall-clock time.
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("interactive call ended early: %v", err)
		default:
		}
		ext.mu.Lock()
		pending := len(ext.pendingTool)
		ext.mu.Unlock()
		if pending != 1 {
			t.Fatal("interactive call is no longer pending")
		}
		sendToolFrame(t, replies, extproto.ToolResultFromExt{Type: "tool_result", ID: call.ID})
		if err := awaitToolError(t, done); err != nil {
			t.Fatal(err)
		}
	})
}

func TestInteractiveToolHeadless(t *testing.T) {
	m := New(t.TempDir(), "", "test", "", "", &stubHooks{})
	tool := NewTool(m, ToolInfo{Name: "ask", Interactive: true})
	result, err := tool.Execute(context.Background(), nil, nil)
	if err != nil || !result.IsError || result.Status != "failed" {
		t.Fatalf("headless result: %+v, %v", result, err)
	}
}

type blockedToolWriter struct {
	started chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func (w *blockedToolWriter) Write([]byte) (int, error) {
	close(w.started)
	<-w.closed
	return 0, io.ErrClosedPipe
}

func (w *blockedToolWriter) Close() error {
	w.once.Do(func() { close(w.closed) })
	return nil
}

func TestToolCancellationDuringWrite(t *testing.T) {
	w := &blockedToolWriter{started: make(chan struct{}), closed: make(chan struct{})}
	pipe := newOrderedPipe(w)
	defer pipe.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := pipe.writeContext(ctx, []byte("{}\n"), time.Hour)
		done <- err
	}()
	select {
	case <-w.started:
	case <-time.After(time.Second):
		t.Fatal("write did not start")
	}
	cancel()
	if err := awaitToolError(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("write cancellation: %v", err)
	}
}
