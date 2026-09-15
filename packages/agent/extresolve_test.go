package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindExtensionDir(t *testing.T) {
	for _, scope := range []string{"global", "project"} {
		t.Run(scope, func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			t.Setenv("ZOT_HOME", home)
			t.Chdir(project)
			root := filepath.Join(home, "extensions")
			if scope == "project" {
				root = filepath.Join(project, ".zot", "extensions")
			}
			dir := writeResolverExtension(t, root, "zot-todo", `{"name":"todo"}`)
			for _, name := range []string{"todo", "zot-todo"} {
				got, err := findExtensionDir(name)
				if err != nil || got != dir {
					t.Fatalf("findExtensionDir(%q) = %q, %v, want %q", name, got, err, dir)
				}
			}
			if _, err := findExtensionDir("missing"); err == nil {
				t.Fatal("expected missing extension error")
			}
			if err := extToggle([]string{"todo"}, false); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
			if err != nil || !strings.Contains(string(raw), `"enabled": false`) {
				t.Fatalf("disable did not update manifest: %s, %v", raw, err)
			}
			if err := extToggle([]string{"todo"}, true); err != nil {
				t.Fatal(err)
			}
			if err := extRemove([]string{"todo", "--yes"}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("extension was not removed: %v", err)
			}
		})
	}
}

func TestFindExtensionDirAmbiguous(t *testing.T) {
	for _, collision := range []string{"manifest", "directory", "cross-scope"} {
		t.Run(collision, func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			t.Setenv("ZOT_HOME", home)
			t.Chdir(project)
			root := filepath.Join(home, "extensions")
			first := writeResolverExtension(t, root, "zot-todo", `{"name":"todo"}`)
			secondName := "other-todo"
			if collision == "directory" {
				secondName = "todo"
			}
			if collision == "cross-scope" {
				root = filepath.Join(project, ".zot", "extensions")
				secondName = "zot-todo"
			}
			second := writeResolverExtension(t, root, secondName, `{"name":"todo"}`)
			if err := extRemove([]string{"todo", "--yes"}); err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("expected ambiguity error, got %v", err)
			}
			for _, dir := range []string{first, second} {
				if _, err := os.Stat(dir); err != nil {
					t.Fatalf("ambiguous removal modified %s: %v", dir, err)
				}
			}
		})
	}
}

func TestFindExtensionDirRejectsTraversal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ZOT_HOME", home)
	t.Chdir(t.TempDir())
	writeResolverExtension(t, home, "outside", `{"name":"outside"}`)
	if _, err := findExtensionDir("../outside"); err == nil {
		t.Fatal("resolved directory outside extension roots")
	}
}

func writeResolverExtension(t *testing.T, root, dir, manifest string) string {
	t.Helper()
	path := filepath.Join(root, dir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
