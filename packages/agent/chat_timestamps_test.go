package agent

import "testing"

func TestChatTimestampSettingsPersistence(t *testing.T) {
	t.Setenv("ZOT_HOME", t.TempDir())
	if err := SaveConfig(Config{Theme: "dark"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil || cfg.ChatTimestamps || cfg.ChatTimestampIntervalMinutes != nil {
		t.Fatalf("unexpected defaults: %+v, %v", cfg, err)
	}
	store := configSettingsStore{}
	for _, mode := range []string{"every_message", "day_start", "", "invalid"} {
		if err := store.SetChatTimestampDate(mode); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig()
		want := "day_start"
		if mode == "every_message" {
			want = mode
		}
		if err != nil || cfg.ChatTimestampDate != want || cfg.Theme != "dark" {
			t.Fatalf("date mode not persisted: %+v, %v", cfg, err)
		}
	}
	for _, enabled := range []bool{true, false} {
		if err := store.SetChatTimestamps(enabled); err != nil {
			t.Fatal(err)
		}
		for _, minutes := range []int{0, 1, 5, 7, -1} {
			if err := store.SetChatTimestampInterval(minutes); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			want := minutes
			if want < 0 {
				want = 0
			}
			if cfg.ChatTimestamps != enabled || cfg.ChatTimestampIntervalMinutes == nil || *cfg.ChatTimestampIntervalMinutes != want || cfg.Theme != "dark" {
				t.Fatalf("settings not preserved: %+v", cfg)
			}
		}
	}
}
