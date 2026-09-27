package tui

import (
	"regexp"
	"strings"
	"testing"
)

// The stripper must treat "38;5;N" / "38;2;R;G;B" as one unit. Pulling the
// color index out on its own (when it happens to look like an ANSI
// background code, e.g. 45, 48, 49 or 101) leaves "\x1b[38;5m", whose leftover
// "5" terminals read as blink, so the token loses its color and blinks.
func TestStripANSIBackgroundsKeepsExtendedColors(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"256 fg in background range", "\x1b[38;5;45mstring\x1b[0m", "\x1b[38;5;45mstring\x1b[0m"},
		{"256 fg index 48", "\x1b[38;5;48mx\x1b[0m", "\x1b[38;5;48mx\x1b[0m"},
		{"256 fg index 49", "\x1b[38;5;49mx\x1b[0m", "\x1b[38;5;49mx\x1b[0m"},
		{"256 fg bright range", "\x1b[38;5;101mcomment\x1b[0m", "\x1b[38;5;101mcomment\x1b[0m"},
		{"256 fg index 107", "\x1b[38;5;107mx\x1b[0m", "\x1b[38;5;107mx\x1b[0m"},
		{"256 fg index 41", "\x1b[38;5;41mx\x1b[0m", "\x1b[38;5;41mx\x1b[0m"},
		{"bold plus 256 fg", "\x1b[1;38;5;45mstring\x1b[0m", "\x1b[1;38;5;45mstring\x1b[0m"},
		{"truecolor fg", "\x1b[38;2;0;229;255mstring\x1b[0m", "\x1b[38;2;0;229;255mstring\x1b[0m"},
		{"plain text", "hello", "hello"},
		{"reset", "\x1b[0m", "\x1b[0m"},

		{"256 bg dropped", "\x1b[48;5;45mbg\x1b[0m", "bg\x1b[0m"},
		{"256 bg dropped, fg kept", "\x1b[38;5;45m\x1b[48;5;0mtext", "\x1b[38;5;45mtext"},
		{"truecolor bg dropped", "\x1b[48;2;9;0;21mbg\x1b[0m", "bg\x1b[0m"},
		{"default bg dropped", "\x1b[49mbg", "bg"},
		{"ansi bg 41 dropped", "\x1b[41mbg", "bg"},
		{"ansi bg 101 dropped", "\x1b[101mbg", "bg"},
		{"truncated fg dropped", "\x1b[38;5mstring", "string"},
		{"truncated bg dropped", "\x1b[48;5mstring", "string"},
		{"truncated fg inside row", "\x1b[1m\x1b[38;5;109mfunc\x1b[0m\x1b[38;5m(", "\x1b[1m\x1b[38;5;109mfunc\x1b[0m("},
	}
	for _, c := range cases {
		if got := stripANSIBackgrounds(c.in); got != c.want {
			t.Errorf("%s:\n in   %q\n got  %q\n want %q", c.name, c.in, got, c.want)
		}
	}
}

// incompleteExtendedColor returns the first SGR sequence in s whose extended
// color introducer ("38" or "48") is missing part of its parameter run, for
// example "\x1b[38;5m", which terminals read as a color-less SGR 38 followed
// by SGR 5 (blink). Validation here is independent of the production helper.
func incompleteExtendedColor(s string) string {
	numeric := func(p string) bool {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	}
	for i := 0; i < len(s); {
		if s[i] != 0x1b || i+1 >= len(s) || s[i+1] != '[' {
			i++
			continue
		}
		j := i + 2
		for j < len(s) && !(s[j] >= 0x40 && s[j] <= 0x7e) {
			j++
		}
		if j >= len(s) {
			return s[i:]
		}
		if s[j] == 'm' {
			params := strings.Split(s[i+2:j], ";")
			for p := 0; p < len(params); p++ {
				if params[p] != "38" && params[p] != "48" {
					continue
				}
				need := 0
				switch {
				case p+1 < len(params) && params[p+1] == "5":
					need = 2
				case p+1 < len(params) && params[p+1] == "2":
					need = 4
				default:
					return s[i : j+1]
				}
				if p+need >= len(params) || !numeric(params[p+2]) {
					return s[i : j+1]
				}
				if need == 4 {
					for k := p + 2; k <= p+4 && k < len(params); k++ {
						if !numeric(params[k]) {
							return s[i : j+1]
						}
					}
				}
				p += need
			}
		}
		i = j + 1
	}
	return ""
}

// A theme color that chroma maps to xterm index 45 ("#00e5ff") must survive
// highlighting with a renderable sequence: a complete "\x1b[38;5;45m", never
// the malformed "\x1b[38;5m".
func TestHighlightCodeExtendedColorIsWellFormed(t *testing.T) {
	th := Dark
	th.Syntax = SyntaxTheme{
		KeywordType: "#00e5ff",
		NameBuiltin: "#00e5ff",
	}
	rows := th.HighlightCode("func splitFrontmatter(raw string) (string, string) {\n", "go")
	joined := strings.Join(rows, "\n")
	if strings.Contains(joined, "\x1b[38;5m") || strings.Contains(joined, "\x1b[48;5m") {
		t.Fatalf("highlighting emitted a malformed extended color: %q", joined)
	}
	if bad := incompleteExtendedColor(joined); bad != "" {
		t.Fatalf("highlighting emitted an incomplete extended color %q: %q", bad, joined)
	}
	// The type keyword keeps a full 256-color foreground.
	idx := strings.Index(joined, "string")
	if idx < 0 {
		t.Fatalf("type keyword missing from highlighted output: %q", joined)
	}
	prefix := joined[:idx]
	esc := prefix[strings.LastIndex(prefix, "\x1b["):]
	if !regexp.MustCompile(`^\x1b\[(?:[0-9]+;)*38;5;[0-9]+m$`).MatchString(esc) {
		t.Fatalf("type keyword lost its color, got %q in %q", esc, joined)
	}
}

// Every syntax color in the built-in palettes must come out of the
// highlighting path as a complete SGR sequence.
func TestHighlightCodeAllPalettesWellFormed(t *testing.T) {
	samples := map[string]string{
		"go":       "package main\n\n// note\nfunc main() { s := \"x\"; _ = len(s) }\n",
		"python":   "def f(x: int) -> str:\n    # note\n    return str(x)\n",
		"markdown": "# Title\n\nSome *emphasis* and **bold** and `code`.\n",
	}
	for _, th := range []Theme{Dark, Light} {
		for lang, src := range samples {
			joined := strings.Join(th.HighlightCode(src, lang), "\n")
			if bad := incompleteExtendedColor(joined); bad != "" {
				t.Errorf("%s emitted an incomplete extended color %q: %q", lang, bad, joined)
			}
		}
	}
}
