package modes

import (
	"strings"
	"testing"

	"github.com/patriceckhart/zot/packages/tui"
)

func TestHelpShowsLlamaOnlyWhenConfigured(t *testing.T) {
	without := strings.Join(renderHelpBlock(tui.Theme{}, 80, false, nil), "\n")
	if strings.Contains(without, "/llama") {
		t.Fatalf("help exposed /llama without login: %q", without)
	}
	with := strings.Join(renderHelpBlock(tui.Theme{}, 80, true, nil), "\n")
	if !strings.Contains(with, "/llama") {
		t.Fatalf("help omitted /llama with login: %q", with)
	}
}

func TestHelpShowsConfiguredCustomKeys(t *testing.T) {
	help := strings.Join(renderHelpBlock(tui.Theme{}, 80, false, map[string]string{
		"ctrl+s": "/skill:review",
	}), "\n")
	if !strings.Contains(help, "custom keys:") || !strings.Contains(help, "ctrl+s") || !strings.Contains(help, "/skill:review") {
		t.Fatalf("help omitted configured custom key: %q", help)
	}
}
