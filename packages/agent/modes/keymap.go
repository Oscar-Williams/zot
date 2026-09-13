package modes

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/patriceckhart/zot/packages/tui"
)

// configuredKeyCommand resolves a configured key chord. Key names are
// case-insensitive and use '+' separated modifiers, for example
// "ctrl+shift+g", "alt+enter", or "cmd+k". Invalid entries are ignored.
func sortedCustomKeys(keymap map[string]string) []string {
	keys := make([]string, 0, len(keymap))
	for key := range keymap {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, right := strings.ToLower(keys[i]), strings.ToLower(keys[j])
		if left == right {
			return keys[i] < keys[j]
		}
		return left < right
	})
	return keys
}

func configuredKeyCommand(keymap map[string]string, key tui.Key) string {
	if len(keymap) == 0 {
		return ""
	}
	keys := make([]string, 0, len(keymap))
	for name := range keymap {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		spec, ok := parseConfiguredKey(name)
		if ok && spec.matches(key) {
			return strings.TrimSpace(keymap[name])
		}
	}
	return ""
}

type configuredKey struct {
	kind      tui.KeyKind
	rune      rune
	ctrl, alt bool
	shift     bool
	super     bool
}

func parseConfiguredKey(value string) (configuredKey, bool) {
	var out configuredKey
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), "+")
	if len(parts) == 0 {
		return out, false
	}
	base := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "ctrl", "control":
			out.ctrl = true
		case "alt", "option", "opt":
			out.alt = true
		case "shift":
			out.shift = true
		case "cmd", "command", "super", "meta", "win":
			out.super = true
		case "":
			return out, false
		default:
			if base != "" {
				return out, false
			}
			base = part
		}
	}
	if base == "" {
		return out, false
	}
	out.kind = tui.KeyRune
	if len([]rune(base)) == 1 {
		// Bare printable keys belong to normal text entry and must not be
		// intercepted by a typo in config.json.
		if !out.ctrl && !out.alt && !out.shift && !out.super {
			return out, false
		}
		out.rune = []rune(base)[0]
		return out, true
	}
	special := map[string]tui.KeyKind{
		"enter": tui.KeyEnter, "return": tui.KeyEnter, "tab": tui.KeyTab,
		"shift-tab": tui.KeyShiftTab, "esc": tui.KeyEsc, "escape": tui.KeyEsc,
		"backspace": tui.KeyBackspace, "delete": tui.KeyDelete,
		"up": tui.KeyUp, "down": tui.KeyDown, "left": tui.KeyLeft, "right": tui.KeyRight,
		"home": tui.KeyHome, "end": tui.KeyEnd, "pageup": tui.KeyPageUp, "pagedown": tui.KeyPageDown,
		"space": tui.KeyRune,
	}
	kind, ok := special[base]
	if !ok {
		return out, false
	}
	// Shift+Tab is represented as a dedicated kind by some terminals and
	// as Tab+Shift by enhanced keyboard protocols; normalize it to the
	// latter so both forms match.
	if kind == tui.KeyShiftTab {
		out.kind = tui.KeyTab
		out.shift = true
	} else {
		out.kind = kind
	}
	if base == "space" {
		out.rune = ' '
	}
	return out, true
}

func (k configuredKey) matches(actual tui.Key) bool {
	kind, r, ctrl := actual.Kind, actual.Rune, actual.Ctrl
	shift := actual.Shift
	if kind == tui.KeyShiftTab {
		kind = tui.KeyTab
		shift = true
	}
	// Raw control bytes are exposed as dedicated KeyKinds by the reader.
	if kind >= tui.KeyCtrlC && kind <= tui.KeyCtrlO {
		ctrl = true
		r = map[tui.KeyKind]rune{
			tui.KeyCtrlC: 'c', tui.KeyCtrlD: 'd', tui.KeyCtrlL: 'l', tui.KeyCtrlU: 'u',
			tui.KeyCtrlK: 'k', tui.KeyCtrlA: 'a', tui.KeyCtrlE: 'e', tui.KeyCtrlW: 'w', tui.KeyCtrlO: 'o',
		}[kind]
		kind = tui.KeyRune
	}
	// Some enhanced keyboard protocols report the shifted glyph rather
	// than the physical number key (Ctrl+Shift+7 becomes '&'). Normalize
	// those common US-layout glyphs so number chords remain bindable.
	if ctrl && shift {
		if digit, ok := shiftedDigit(r); ok {
			r = digit
		}
	}
	// Enhanced terminal protocols preserve the uppercase glyph for shifted
	// letters, while configured key names are case-insensitive.
	r = unicode.ToLower(r)
	wantRune := unicode.ToLower(k.rune)
	// Legacy terminals do not encode Shift on control letters, so a
	// Ctrl+Shift+letter binding must also accept the corresponding raw
	// Ctrl+letter event. Enhanced protocols still match the exact chord.
	shiftMatches := shift == k.shift ||
		(k.ctrl && k.shift && !shift && ctrl && r >= 'a' && r <= 'z')
	return kind == k.kind && r == wantRune && ctrl == k.ctrl && actual.Alt == k.alt &&
		shiftMatches && actual.Super == k.super
}

func shiftedDigit(r rune) (rune, bool) {
	switch r {
	case ')', '!', '@', '#', '$', '%', '^', '&', '*', '(':
		return []rune("0123456789")[strings.IndexRune(")!@#$%^&*(", r)], true
	default:
		return 0, false
	}
}

func keymapReservedKey(k tui.Key) bool {
	// Keep the emergency/exit controls authoritative even when a stale or
	// overly broad config.json attempts to bind them.
	switch k.Kind {
	case tui.KeyCtrlC, tui.KeyCtrlD:
		return true
	case tui.KeyEsc:
		return !k.Ctrl && !k.Alt && !k.Shift && !k.Super
	}
	return false
}

func validateKeymap(keymap map[string]string) error {
	for key, command := range keymap {
		if _, ok := parseConfiguredKey(key); !ok {
			return fmt.Errorf("invalid keymap key %q", key)
		}
		if strings.TrimSpace(command) == "" {
			return fmt.Errorf("empty keymap command for %q", key)
		}
	}
	return nil
}
