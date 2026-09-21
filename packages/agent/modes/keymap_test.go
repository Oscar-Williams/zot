package modes

import (
	"context"
	"strings"
	"testing"

	"github.com/patriceckhart/zot/packages/agent/tools"
	"github.com/patriceckhart/zot/packages/core"
	"github.com/patriceckhart/zot/packages/tui"
)

func TestConfiguredKeyCommandMatchesModifiers(t *testing.T) {
	keymap := map[string]string{"Ctrl+Shift+G": "/git-tools:status"}
	if got := configuredKeyCommand(keymap, tui.Key{Kind: tui.KeyRune, Rune: 'g', Ctrl: true, Shift: true}); got != "/git-tools:status" {
		t.Fatalf("configuredKeyCommand = %q, want extension command", got)
	}
	if got := configuredKeyCommand(keymap, tui.Key{Kind: tui.KeyRune, Rune: 'G', Ctrl: true, Shift: true}); got != "/git-tools:status" {
		t.Fatalf("shifted uppercase configuredKeyCommand = %q, want extension command", got)
	}
	if got := configuredKeyCommand(keymap, tui.Key{Kind: tui.KeyRune, Rune: 'g', Ctrl: true}); got != "/git-tools:status" {
		t.Fatalf("legacy ctrl-letter configuredKeyCommand = %q, want extension command", got)
	}
	if got := configuredKeyCommand(keymap, tui.Key{Kind: tui.KeyRune, Rune: 'g', Shift: true}); got != "" {
		t.Fatalf("unmodified key matched command %q", got)
	}
}

func TestConfiguredKeyCommandPrefersExactChord(t *testing.T) {
	keymap := map[string]string{
		"ctrl+g":       "/help",
		"ctrl+shift+g": "/git-tools:status",
	}
	if got := configuredKeyCommand(keymap, tui.Key{Kind: tui.KeyRune, Rune: 'g', Ctrl: true}); got != "/help" {
		t.Fatalf("plain ctrl+g = %q, want exact /help binding", got)
	}
	if got := configuredKeyCommand(keymap, tui.Key{Kind: tui.KeyRune, Rune: 'G', Ctrl: true, Shift: true}); got != "/git-tools:status" {
		t.Fatalf("ctrl+shift+g = %q, want /git-tools:status", got)
	}
}

func TestConfiguredKeyCommandMatchesDedicatedControlKeys(t *testing.T) {
	if got := configuredKeyCommand(map[string]string{"ctrl+l": "/skill:review"}, tui.Key{Kind: tui.KeyCtrlL}); got != "/skill:review" {
		t.Fatalf("configuredKeyCommand = %q, want skill command", got)
	}
	if got := configuredKeyCommand(map[string]string{"ctrl+h": "/help"}, tui.Key{Kind: tui.KeyRune, Rune: 'h', Ctrl: true}); got != "/help" {
		t.Fatalf("enhanced ctrl+h binding = %q, want /help", got)
	}
}

func TestParseConfiguredKeyRejectsMalformedAndSupportsSpecialKeys(t *testing.T) {
	for _, bad := range []string{"ctrl+shift", "g", "enter", "space", "tab", "up", "backspace", "no-such-key", "ctrl+", "ctrl+a+b", ""} {
		if _, ok := parseConfiguredKey(bad); ok {
			t.Fatalf("parseConfiguredKey(%q) accepted an unbindable key", bad)
		}
	}
	key, ok := parseConfiguredKey("alt+enter")
	if !ok || key.kind != tui.KeyEnter || !key.alt {
		t.Fatalf("parseConfiguredKey(alt+enter) = %#v, %v", key, ok)
	}
	if got := configuredKeyCommand(map[string]string{"shift+tab": "/help"}, tui.Key{Kind: tui.KeyShiftTab}); got != "/help" {
		t.Fatalf("shift+tab binding = %q, want /help", got)
	}
	if got := configuredKeyCommand(map[string]string{"ctrl+space": "/help"}, tui.Key{Kind: tui.KeyRune, Rune: ' ', Ctrl: true}); got != "/help" {
		t.Fatalf("ctrl+space binding = %q, want /help", got)
	}
}

func TestCompileKeymapReportsInvalidAndReservedEntries(t *testing.T) {
	bindings, issues := compileKeymap(map[string]string{
		"ctrl+c":      "/help",
		"ctrl+d":      "/help",
		"esc":         "/help",
		"enter":       "/help",
		"ctrl+s":      "settings",
		"ctrl+x":      "",
		"alt+r":       "/skill:code-review",
		"ctrl+alt+c":  "/help",
		"Shift+Enter": "/help",
	})
	if len(issues) != 6 {
		t.Fatalf("issues = %v, want 6 entries", issues)
	}
	for _, issue := range issues {
		if !strings.HasPrefix(issue, "keymap: ") {
			t.Fatalf("issue %q lacks keymap prefix", issue)
		}
	}
	var names []string
	for _, b := range bindings {
		names = append(names, b.name)
	}
	if got, want := strings.Join(names, ","), "alt+r,ctrl+alt+c,shift+enter"; got != want {
		t.Fatalf("compiled bindings = %q, want %q", got, want)
	}
	if got := lookupKeymap(bindings, tui.Key{Kind: tui.KeyCtrlC}); got != "" {
		t.Fatalf("ctrl+c resolved to %q, want reserved", got)
	}
	if got := lookupKeymap(bindings, tui.Key{Kind: tui.KeyEnter, Shift: true}); got != "/help" {
		t.Fatalf("shift+enter = %q, want /help", got)
	}
}

func newKeymapInteractive(keymap map[string]string) *Interactive {
	agent := core.NewAgent(nil, "test-model", "test system", nil)
	return NewInteractive(InteractiveConfig{
		Theme:    tui.Dark,
		Terminal: tui.NewProcTerm(),
		Agent:    agent,
		Sandbox:  tools.NewSandbox("."),
		Keymap:   keymap,
	})
}

func TestInteractiveKeymapRunsSlashCommand(t *testing.T) {
	i := newKeymapInteractive(map[string]string{"ctrl+s": "/help", "bogus": "/help"})
	if len(i.reloadErrors) != 1 || !strings.Contains(i.reloadErrors[0], `"bogus"`) {
		t.Fatalf("reloadErrors = %v, want one keymap diagnostic", i.reloadErrors)
	}
	i.ed.SetValue("draft")
	i.handleKey(context.Background(), tui.Key{Kind: tui.KeyRune, Rune: 's', Ctrl: true})
	if len(i.helpBlock) == 0 {
		t.Fatal("ctrl+s did not run /help")
	}
	if got := i.ed.Value(); got != "" {
		t.Fatalf("editor = %q, want cleared before running the bound command", got)
	}
}

func TestInteractiveUnboundChordDoesNotType(t *testing.T) {
	i := newKeymapInteractive(nil)
	i.handleKey(context.Background(), tui.Key{Kind: tui.KeyRune, Rune: 's', Ctrl: true})
	i.handleKey(context.Background(), tui.Key{Kind: tui.KeyRune, Rune: 'k', Super: true})
	if got := i.ed.Value(); got != "" {
		t.Fatalf("editor = %q, want unbound chords ignored", got)
	}
}

func TestInteractiveKeymapInactiveWhileConfirmationFocused(t *testing.T) {
	i := newKeymapInteractive(map[string]string{"ctrl+s": "/help"})
	resp := enqueueConfirmation(i)
	i.handleKey(context.Background(), tui.Key{Kind: tui.KeyRune, Rune: 's', Ctrl: true})
	if len(i.helpBlock) != 0 {
		t.Fatal("keymap fired while confirmation owned input")
	}
	select {
	case decision := <-resp:
		t.Fatalf("chord answered confirmation: %+v", decision)
	default:
	}
}

func TestInteractiveChordDoesNotLeakIntoDialogAsLetter(t *testing.T) {
	i := newKeymapInteractive(map[string]string{"ctrl+r": "/help"})
	i.sessionDialog.sessions = []core.SessionSummary{{Path: "a.jsonl", Title: "one", MessageCount: 1}}
	i.sessionDialog.active = true
	i.handleKey(context.Background(), tui.Key{Kind: tui.KeyRune, Rune: 'r', Ctrl: true})
	if i.sessionDialog.renaming {
		t.Fatal("ctrl+r started a rename as if it were a bare 'r'")
	}
	if len(i.helpBlock) != 0 {
		t.Fatal("keymap fired while the session dialog owned input")
	}
	if !i.sessionDialog.Active() {
		t.Fatal("session dialog closed unexpectedly")
	}
}
