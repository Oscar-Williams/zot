package tui

import (
	"strings"
	"testing"

	"github.com/patriceckhart/zot/packages/provider"
)

func TestExpandedCompactionWrapsAfterResize(t *testing.T) {
	for _, text := range []string{
		strings.Repeat("summary words ", 20),
		"**" + strings.Repeat("styled text ", 20) + "**",
		"```go\n" + strings.Repeat("longIdentifier", 20) + "\n```",
		strings.Repeat("日本語 Grüße ", 20),
	} {
		v := View{
			Theme: Dark,
			Messages: []provider.Message{{
				Role: provider.RoleUser,
				Meta: map[string]string{"compaction": "true", "tokens_before": "12345"},
				Content: []provider.Content{provider.TextBlock{
					Text: "## Context Summary\n\n" + text,
				}},
			}},
		}
		if got := strings.TrimSpace(strings.Join(v.Build(120), "")); got != "" {
			t.Fatalf("collapsed summary should be hidden: %q", got)
		}
		v.ExpandAll = true
		canonical := func(s string) string {
			return strings.Join(strings.Fields(stripANSI(s)), "")
		}
		want := canonical("compacted from ~12345 tokens" + RenderMarkdown(text, Dark, 116))
		for _, width := range []int{120, 40, 12, 4, 2, 120} {
			lines := v.Build(width)
			for _, line := range lines {
				if got := visibleWidth(line); got > width {
					t.Fatalf("row width %d exceeds terminal width %d: %q", got, width, stripANSI(line))
				}
			}
			if got := canonical(strings.Join(lines, "")); got != want {
				t.Fatalf("width %d lost summary content\ngot: %s\nwant: %s", width, got, want)
			}
		}
	}
}
