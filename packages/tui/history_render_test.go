package tui

import (
	"bytes"
	"strings"
	"testing"
)

func TestHistoryPreservesMainScreen(t *testing.T) {
	for _, program := range []string{"", "vscode"} {
		t.Run(program, func(t *testing.T) {
			t.Setenv("TERM_PROGRAM", program)
			var out bytes.Buffer
			r := NewRenderer(&out)
			r.Resize(80, 24)
			chat := []string{"original transcript", "live tail"}
			bottom := []string{"editor"}
			r.DrawLog(chat, bottom, 0, 2)
			out.Reset()
			r.DrawHistory([]string{"selected turn"}, bottom, 0, 2)
			r.Clear()
			r.DrawHistory([]string{"another turn"}, bottom, 0, 2)
			if strings.Count(out.String(), SeqAltScreenOn) != 1 || strings.Contains(out.String(), SeqClearScrollback) {
				t.Fatalf("history reset native scrollback or re-entered alternate screen: %q", out.String())
			}
			if !strings.Contains(out.String(), "selected turn") || !strings.Contains(out.String(), "another turn") {
				t.Fatal("history content not rendered")
			}
			out.Reset()
			r.DrawLog(chat, bottom, 0, 2)
			if out.String() != SeqAltScreenOff {
				t.Fatalf("unchanged main screen was replayed: %q", out.String())
			}
			out.Reset()
			r.DrawHistory([]string{"history"}, bottom, 0, 2)
			out.Reset()
			r.DrawLog(append(chat, "new streamed output"), bottom, 0, 2)
			if !strings.HasPrefix(out.String(), SeqAltScreenOff) || !strings.Contains(out.String(), "new streamed output") || strings.Contains(out.String(), "original transcript") {
				t.Fatalf("return from history did not append new output: %q", out.String())
			}
		})
	}
}

func TestHistoryResizeAndClose(t *testing.T) {
	var out bytes.Buffer
	r := NewRenderer(&out)
	r.Resize(80, 24)
	r.DrawLog([]string{"tail"}, []string{"editor"}, 0, 0)
	r.DrawHistory([]string{"history"}, []string{"editor"}, 0, 0)
	out.Reset()
	r.Resize(40, 12)
	r.DrawHistory([]string{"resized history"}, []string{"editor"}, 0, 0)
	if r.history.cols != 40 || r.history.rows != 12 || !strings.Contains(out.String(), "resized history") || strings.Contains(out.String(), SeqClearScrollback) {
		t.Fatalf("history resize failed: %q", out.String())
	}
	out.Reset()
	r.CloseHistory()
	r.CloseHistory()
	if out.String() != SeqAltScreenOff {
		t.Fatalf("shutdown must restore main screen exactly once: %q", out.String())
	}
}
