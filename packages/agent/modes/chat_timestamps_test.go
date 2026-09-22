package modes

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/patriceckhart/zot/packages/core"
	"github.com/patriceckhart/zot/packages/provider"
	"github.com/patriceckhart/zot/packages/tui"
)

type timestampSettingsTestStore struct {
	SettingsStore
	enabled  bool
	minutes  int
	dateMode string
	err      error
}

func (s *timestampSettingsTestStore) SetChatTimestamps(enabled bool) error {
	s.enabled = enabled
	return s.err
}

func (s *timestampSettingsTestStore) SetChatTimestampInterval(minutes int) error {
	s.minutes = minutes
	return s.err
}

func (s *timestampSettingsTestStore) SetChatTimestampDate(mode string) error {
	s.dateMode = mode
	return s.err
}

func TestChatTimestampDateSetting(t *testing.T) {
	store := &timestampSettingsTestStore{}
	i := NewInteractive(InteractiveConfig{SettingsStore: store})
	item := i.chatTimestampDateSetting()
	if item.options[item.choice].value != "day_start" {
		t.Fatal("incorrect date default")
	}
	before, _ := i.chatCacheKeyLocked(80)
	i.applySettingChange(settingsAction{Key: "chat_timestamp_date", StringValue: "every_message"})
	if store.dateMode != "every_message" || i.view.ChatTimestampDate != "every_message" || i.cfg.ChatTimestampDate != "every_message" {
		t.Fatal("date mode not applied or persisted")
	}
	after, _ := i.chatCacheKeyLocked(80)
	if before == after {
		t.Fatal("date setting missing from cache key")
	}
	store.err = errors.New("cannot save")
	i.applyChatTimestampDate("day_start")
	if i.view.ChatTimestampDate != "every_message" || i.statusErr == "" {
		t.Fatal("failed save changed live date setting")
	}
	restarted := NewInteractive(InteractiveConfig{ChatTimestampDate: "every_message"})
	if restarted.view.ChatTimestampDate != "every_message" {
		t.Fatal("saved date mode not restored")
	}
}

func TestChatTimestampSettings(t *testing.T) {
	store := &timestampSettingsTestStore{}
	i := NewInteractive(InteractiveConfig{SettingsStore: store})
	if i.view.ChatTimestamps || tui.ChatTimestampInterval(i.view.ChatTimestampIntervalMinutes) != 0 {
		t.Fatal("incorrect startup defaults")
	}
	i.openSettingsDialog()
	found := false
	for idx, item := range i.settingsDialog.items {
		if item.key == "chat_timestamps" || item.key == "chat_timestamp_interval_minutes" {
			t.Fatal("timestamp controls must not be top-level settings")
		}
		if item.key == "timestamps" {
			found = true
			i.settingsDialog.cursor = idx
		}
	}
	if !found {
		t.Fatal("missing timestamps submenu")
	}
	i.settingsDialog.HandleKey(tui.Key{Kind: tui.KeyEnter})
	if i.settingsDialog.title != "settings: timestamps" || len(i.settingsDialog.items) != 3 {
		t.Fatal("timestamps submenu did not open")
	}
	if item := i.settingsDialog.items[0]; item.key != "chat_timestamps" || item.value {
		t.Fatal("missing disabled-by-default toggle")
	}
	item := i.settingsDialog.items[1]
	if item.key != "chat_timestamp_interval_minutes" || item.options[item.choice].value != "0" {
		t.Fatal("incorrect default interval selection")
	}
	dateItem := i.settingsDialog.items[2]
	if dateItem.key != "chat_timestamp_date" || dateItem.options[dateItem.choice].value != "day_start" {
		t.Fatal("missing date display setting")
	}
	i.settingsDialog.HandleKey(tui.Key{Kind: tui.KeyEsc})
	if !i.settingsDialog.Active() || i.settingsDialog.title != "settings" {
		t.Fatal("escape should return to parent settings")
	}
	before, _ := i.chatCacheKeyLocked(80)
	i.applySettingChange(settingsAction{Key: "chat_timestamps", Value: true})
	if !store.enabled || !i.cfg.ChatTimestamps || !i.view.ChatTimestamps {
		t.Fatal("toggle not persisted or applied")
	}
	after, _ := i.chatCacheKeyLocked(80)
	if before == after {
		t.Fatal("toggle did not affect chat cache key")
	}
	i.applySettingChange(settingsAction{Key: "chat_timestamp_interval_minutes", StringValue: "0"})
	if store.minutes != 0 || tui.ChatTimestampInterval(i.view.ChatTimestampIntervalMinutes) != 0 {
		t.Fatal("zero interval not applied")
	}
	for _, value := range []string{"-1", "invalid"} {
		i.applyChatTimestampInterval(value)
		if tui.ChatTimestampInterval(i.view.ChatTimestampIntervalMinutes) != 0 {
			t.Fatal("invalid interval changed state")
		}
	}
	store.err = errors.New("cannot save")
	i.applyChatTimestamps(false)
	i.applyChatTimestampInterval("10")
	if !i.view.ChatTimestamps || !i.cfg.ChatTimestamps || tui.ChatTimestampInterval(i.view.ChatTimestampIntervalMinutes) != 0 || i.statusErr == "" {
		t.Fatal("failed save changed live state or lost the error")
	}
}

func TestChatTimestampsSurviveSessionResume(t *testing.T) {
	root := t.TempDir()
	session, err := core.NewSession(root, root, "ollama", "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	start := time.Date(2026, 9, 22, 12, 0, 0, 0, time.Local)
	messages := []provider.Message{
		{Role: provider.RoleUser, Time: start, Content: []provider.Content{provider.TextBlock{Text: "hello"}}},
		{Role: provider.RoleAssistant, Time: start.Add(5 * time.Minute), Content: []provider.Content{provider.TextBlock{Text: "world"}}},
		{Role: provider.RoleUser, Content: []provider.Content{provider.TextBlock{Text: "legacy message"}}},
	}
	for _, message := range messages {
		if err := session.AppendMessage(message); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, restored, err := core.OpenSession(session.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	before := tui.View{Theme: tui.Dark, ChatTimestamps: true, Messages: messages}
	after := tui.View{Theme: tui.Dark, ChatTimestamps: true, Messages: restored}
	if !reflect.DeepEqual(before.Build(80), after.Build(80)) {
		t.Fatal("resumed transcript changed timestamps")
	}
}

func TestChatTimestampCustomIntervalStartup(t *testing.T) {
	minutes := 7
	i := NewInteractive(InteractiveConfig{ChatTimestamps: true, ChatTimestampIntervalMinutes: &minutes})
	if !i.view.ChatTimestamps || tui.ChatTimestampInterval(i.view.ChatTimestampIntervalMinutes) != 7 {
		t.Fatal("saved settings not applied on startup")
	}
	item := i.chatTimestampIntervalSetting()
	if item.options[item.choice].value != "7" {
		t.Fatal("custom interval not represented in picker")
	}
	before, _ := i.chatCacheKeyLocked(80)
	i.view.ChatTimestampIntervalMinutes = nil
	after, _ := i.chatCacheKeyLocked(80)
	if before == after {
		t.Fatal("interval missing from cache key")
	}
}
