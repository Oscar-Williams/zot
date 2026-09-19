package modes

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/patriceckhart/zot/packages/core"
	"github.com/patriceckhart/zot/packages/tui"
)

func TestReasoningSelectorUsesCurrentModelLevels(t *testing.T) {
	i := &Interactive{cfg: InteractiveConfig{
		Provider:  "openai",
		Model:     "gpt-5.6-sol",
		Reasoning: "minimum",
	}}

	item := i.reasoningSettingItem()
	var levels []string
	for _, option := range item.options {
		levels = append(levels, option.value)
	}
	want := []string{"", "low", "medium", "high", "xhigh", "max"}
	if !slices.Equal(levels, want) {
		t.Fatalf("reasoning levels = %q, want %q", levels, want)
	}
	if item.options[item.choice].value != "low" {
		t.Fatalf("clamped current level = %q, want low", item.options[item.choice].value)
	}
}

func TestReasoningSelectorOnlyOffersOffForUnsupportedModel(t *testing.T) {
	i := &Interactive{cfg: InteractiveConfig{
		Provider:  "google",
		Model:     "gemini-2.0-flash",
		Reasoning: "high",
	}}

	item := i.reasoningSettingItem()
	if len(item.options) != 1 || item.options[0].value != "" {
		t.Fatalf("reasoning levels = %#v, want off only", item.options)
	}
	if item.hint != "current model does not support reasoning" {
		t.Fatalf("hint = %q", item.hint)
	}
}

func TestReasoningCommandOpensDirectSelector(t *testing.T) {
	i := &Interactive{
		cfg: InteractiveConfig{
			Reasoning: "high",
		},
		settingsDialog: newSettingsDialog(),
	}

	i.runSlash(context.Background(), "/reasoning")

	if !i.settingsDialog.Active() || !i.settingsDialog.selecting || !i.settingsDialog.direct {
		t.Fatal("reasoning command did not open the direct option selector")
	}
	lines := i.settingsDialog.Render(tui.Dark, 100)
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "reasoning level") || !strings.Contains(text, "high") {
		t.Fatalf("reasoning selector missing current level: %q", text)
	}
	i.settingsDialog.HandleKey(tui.Key{Kind: tui.KeyDown})
	act := i.settingsDialog.HandleKey(tui.Key{Kind: tui.KeyEnter})
	i.applySettingChange(act)
	if i.cfg.Reasoning != "xhigh" || i.settingsDialog.Active() {
		t.Fatalf("reasoning selection was not applied and closed: level=%q active=%v", i.cfg.Reasoning, i.settingsDialog.Active())
	}
	if i.statusOK != "reasoning level xhigh" {
		t.Fatalf("reasoning status = %q", i.statusOK)
	}

	i.runSlash(context.Background(), "/reasoning")
	act = i.settingsDialog.HandleKey(tui.Key{Kind: tui.KeyEsc})
	if !act.Close || i.settingsDialog.Active() {
		t.Fatal("escape did not close the direct reasoning selector")
	}
}

func TestReasoningSettingNotifiesRuntime(t *testing.T) {
	i := &Interactive{agent: &core.Agent{Reasoning: "high"}}
	var levels []string
	i.cfg.OnReasoningChanged = func(level string) {
		if i.agent.Reasoning != level {
			t.Fatalf("callback ran before agent update: got %q, want %q", i.agent.Reasoning, level)
		}
		levels = append(levels, level)
	}
	i.applyReasoningSetting("medium")
	i.applyReasoningSetting("off")
	if !slices.Equal(levels, []string{"medium", ""}) {
		t.Fatalf("reasoning notifications = %q", levels)
	}
}

func TestNormalizeModelQueryKeepsPresetAtSign(t *testing.T) {
	if got := normalizeModelQuery("@preset/flash"); got != "@presetflash" {
		t.Fatalf("id = %q; want @presetflash", got)
	}
	if got := normalizeModelQuery("preset"); got != "preset" {
		t.Fatalf("query = %q; want preset", got)
	}
	haystack := normalizeModelQuery("openrouter @preset/flash Flash (preset)")
	if !strings.Contains(haystack, normalizeModelQuery("preset")) {
		t.Fatalf("%q does not contain preset", haystack)
	}
}

func TestModelDialogAdvertisesReasoningSelector(t *testing.T) {
	d := newModelDialog()
	d.Open("", nil, "high")

	text := strings.Join(d.Render(tui.Dark, 100), "\n")
	if !strings.Contains(text, "current reasoning: high") || !strings.Contains(text, "/reasoning") {
		t.Fatalf("model dialog missing reasoning guidance: %q", text)
	}
}

func TestModelPickerSkipsLlamaRefreshWhenRouterIsNotConfigured(t *testing.T) {
	i := &Interactive{
		cfg: InteractiveConfig{
			RefreshLlamaCPPModels: func(context.Context) error { return nil },
		},
		modelRefresh: make(chan modelRefreshResult, 1),
		modelDialog:  newModelDialog(),
	}

	i.runSlash(context.Background(), "/model")

	if !i.modelDialog.Active() {
		t.Fatal("model picker did not open immediately")
	}
	if i.statusOK == "refreshing models" {
		t.Fatal("refresh status shown without llama.cpp router configuration")
	}
}

func TestModelPickerRefreshesDiscoverableCustomProviders(t *testing.T) {
	refreshed := make(chan struct{}, 1)
	i := &Interactive{
		cfg: InteractiveConfig{
			CustomDiscoveryConfigured: func() bool { return true },
			RefreshCustomProviderModels: func(context.Context) error {
				refreshed <- struct{}{}
				return nil
			},
		},
		modelRefresh: make(chan modelRefreshResult, 1),
		modelDialog:  newModelDialog(),
	}

	i.runSlash(context.Background(), "/model")

	if i.modelDialog.Active() {
		t.Fatal("model picker opened before the custom provider refresh finished")
	}
	if i.statusOK != "refreshing models" {
		t.Fatalf("status = %q", i.statusOK)
	}
	select {
	case <-refreshed:
	case <-time.After(5 * time.Second):
		t.Fatal("custom provider refresh was not invoked")
	}
	select {
	case result := <-i.modelRefresh:
		i.openModelPickerAfterRefresh(result.err)
	case <-time.After(5 * time.Second):
		t.Fatal("refresh result never delivered")
	}
	if !i.modelDialog.Active() {
		t.Fatal("model picker did not open after refresh")
	}

	// Without opt-in, the picker opens immediately even when the hook exists.
	i = &Interactive{
		cfg: InteractiveConfig{
			CustomDiscoveryConfigured:   func() bool { return false },
			RefreshCustomProviderModels: func(context.Context) error { t.Error("unexpected refresh"); return nil },
		},
		modelRefresh: make(chan modelRefreshResult, 1),
		modelDialog:  newModelDialog(),
	}
	i.runSlash(context.Background(), "/model")
	if !i.modelDialog.Active() {
		t.Fatal("model picker did not open immediately")
	}
}
