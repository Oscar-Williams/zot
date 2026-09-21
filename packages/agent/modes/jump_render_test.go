package modes

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/patriceckhart/zot/packages/agent/tools"
	"github.com/patriceckhart/zot/packages/core"
	"github.com/patriceckhart/zot/packages/provider"
	"github.com/patriceckhart/zot/packages/tui"
)

func TestJumpRendersSelectedTurn(t *testing.T) {
	for _, tc := range []struct {
		name            string
		tailLimit, turn int
		filtered        bool
	}{
		{"picker", 0, 1, false},
		{"resumed older turn", 80, 10, false},
		{"filtered shortcut", 80, 50, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			term := &cleanupTestTerminal{}
			agent := core.NewAgent(nil, "test", "", nil)
			var msgs []provider.Message
			for n := 1; n <= 100; n++ {
				msgs = append(msgs, provider.Message{Role: provider.RoleUser, Content: []provider.Content{provider.TextBlock{Text: fmt.Sprintf("prompt-number-%03d", n)}}})
			}
			agent.SetMessages(msgs)
			i := NewInteractive(InteractiveConfig{Terminal: term, Agent: agent, Sandbox: tools.NewSandbox(t.TempDir())})
			i.rend.Resize(80, 24)
			i.view.TailLimit = tc.tailLimit
			i.redraw()
			targetText := fmt.Sprintf("prompt-number-%03d", tc.turn)
			if tc.filtered {
				term.Reset()
				i.openJumpDialog([]string{targetText})
			} else {
				i.openJumpDialog(nil)
				i.redraw()
				target := i.jumpDialog.Targets()[tc.turn-1]
				i.jumpDialog.Close()
				term.Reset()
				i.applyJumpSelection(target.MessageIdx, target.TurnNo)
			}
			i.redraw()
			output := term.String()
			if !strings.Contains(output, tui.SeqAltScreenOn) || !strings.Contains(output, targetText) || !strings.Contains(output, fmt.Sprintf("viewing turn %d of 100", tc.turn)) {
				t.Fatalf("selected turn not rendered in history: %q", output)
			}
			if strings.Contains(output, "prompt-number-100") {
				t.Fatal("history still displays the live tail")
			}
			if !reflect.DeepEqual(agent.Messages(), msgs) {
				t.Fatal("jump modified the transcript")
			}
			// Appending a completed turn must not move the history viewport.
			beforeChat := i.cachedChatLocked(80)
			top := len(beforeChat) - i.scrollOffset - i.prevChatRows
			_, anchors := i.view.BuildWithAnchors(80)
			if top != anchors[tc.turn-1].Row {
				t.Fatalf("jump top = %d, want target row %d", top, anchors[tc.turn-1].Row)
			}
			agent.SetMessages(append(msgs, provider.Message{Role: provider.RoleUser, Content: []provider.Content{provider.TextBlock{Text: "new output"}}}))
			i.redraw()
			afterChat := i.cachedChatLocked(80)
			if got := len(afterChat) - i.scrollOffset - i.prevChatRows; got != top {
				t.Fatalf("history moved while output arrived: %d to %d", top, got)
			}
			term.Reset()
			for pages := 0; i.scrollOffset > 0 && pages < 100; pages++ {
				i.scrollBy(-i.chatPage())
				i.redraw()
			}
			if i.scrollOffset != 0 {
				t.Fatal("page down did not reach the live tail")
			}
			if !strings.Contains(term.String(), tui.SeqAltScreenOff) || !strings.Contains(term.String(), "new output") {
				t.Fatal("return to live tail did not restore main screen with new output")
			}
		})
	}
}
