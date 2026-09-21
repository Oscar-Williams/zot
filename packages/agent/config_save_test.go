package agent

import (
	"os"
	"testing"
)

func TestSaveConfigPreservesKeymapForPartialConfig(t *testing.T) {
	t.Setenv("ZOT_HOME", t.TempDir())

	keymap := map[string]string{"ctrl+shift+g": "/git-tools:status"}
	if err := SaveConfig(Config{Theme: "dark"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ConfigPath(), []byte(`{"theme":"dark","keymap":{"ctrl+shift+g":"/git-tools:status"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(Config{Theme: "light"}); err != nil {
		t.Fatal(err)
	}

	got, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Keymap) != 1 || got.Keymap["ctrl+shift+g"] != keymap["ctrl+shift+g"] {
		t.Fatalf("keymap = %#v, want %#v", got.Keymap, keymap)
	}
	if got.Theme != "light" {
		t.Fatalf("theme = %q, want %q", got.Theme, "light")
	}
}
