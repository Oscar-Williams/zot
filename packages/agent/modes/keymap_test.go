package modes

import (
	"testing"

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

func TestConfiguredKeyCommandMatchesDedicatedControlKeys(t *testing.T) {
	if got := configuredKeyCommand(map[string]string{"ctrl+c": "/skill:review"}, tui.Key{Kind: tui.KeyCtrlC}); got != "/skill:review" {
		t.Fatalf("configuredKeyCommand = %q, want skill command", got)
	}
	if got := configuredKeyCommand(map[string]string{"ctrl+h": "/help"}, tui.Key{Kind: tui.KeyRune, Rune: 'h', Ctrl: true}); got != "/help" {
		t.Fatalf("enhanced ctrl+h binding = %q, want /help", got)
	}
}

func TestParseConfiguredKeyRejectsMalformedAndSupportsSpecialKeys(t *testing.T) {
	if _, ok := parseConfiguredKey("ctrl+shift"); ok {
		t.Fatal("modifier-only key was accepted")
	}
	if _, ok := parseConfiguredKey("g"); ok {
		t.Fatal("bare printable key was accepted")
	}
	key, ok := parseConfiguredKey("alt+enter")
	if !ok || key.kind != tui.KeyEnter || !key.alt {
		t.Fatalf("parseConfiguredKey(alt+enter) = %#v, %v", key, ok)
	}
	if got := configuredKeyCommand(map[string]string{"shift+tab": "/help"}, tui.Key{Kind: tui.KeyShiftTab}); got != "/help" {
		t.Fatalf("shift+tab binding = %q, want /help", got)
	}
	if err := validateKeymap(map[string]string{"no-such-key": "/help"}); err == nil {
		t.Fatal("invalid keymap was accepted")
	}
}
