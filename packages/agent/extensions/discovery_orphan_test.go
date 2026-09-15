package extensions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiscoverIgnoresStateOnlyDirectories(t *testing.T) {
	for _, globalInstalled := range []bool{false, true} {
		name := "orphan-only"
		if globalInstalled {
			name = "orphan-does-not-shadow-global"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			orphan := filepath.Join(project, ".zot", "extensions", "zot-loop")
			writeDiscoveryFile(t, orphan, "state.json", `{}`)
			global := filepath.Join(home, "extensions", "zot-loop")
			if globalInstalled {
				writeDiscoveryFile(t, global, "extension.json", `{"name":"loop"}`)
				writeDiscoveryFile(t, global, "theme.json", `{"name":"Loop Theme","colors":{"dark":{"accent":204}}}`)
			} else {
				writeDiscoveryFile(t, filepath.Join(home, "extensions", "other-orphan"), "state.json", `{}`)
			}
			mgr := New(home, project, "test", "", "", nil)
			t.Cleanup(func() { mgr.Stop(time.Second) })
			if errs := mgr.Discover(context.Background()); len(errs) != 0 {
				t.Fatalf("discover errors: %v", errs)
			}
			if globalInstalled {
				opts := mgr.ThemeOptions()
				if len(opts) != 1 || opts[0].Path != filepath.Join(global, "theme.json") {
					t.Fatalf("global extension was shadowed: %#v", opts)
				}
			}
			if _, err := os.Stat(filepath.Join(orphan, "state.json")); err != nil {
				t.Fatalf("discovery must not delete leftover state: %v", err)
			}
			if errs := mgr.LoadExplicit(context.Background(), []string{orphan}); len(errs) != 1 || !strings.Contains(errs[0].Error(), "extension.json") {
				t.Fatalf("explicit missing manifest errors: %v", errs)
			}
		})
	}
}

func TestDiscoverStillReportsMalformedManifest(t *testing.T) {
	home := t.TempDir()
	writeDiscoveryFile(t, filepath.Join(home, "extensions", "broken"), "extension.json", `{`)
	mgr := New(home, "", "test", "", "", nil)
	t.Cleanup(func() { mgr.Stop(time.Second) })
	if errs := mgr.Discover(context.Background()); len(errs) != 1 || !strings.Contains(errs[0].Error(), "parse manifest") {
		t.Fatalf("malformed manifest errors: %v", errs)
	}
}

func writeDiscoveryFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
