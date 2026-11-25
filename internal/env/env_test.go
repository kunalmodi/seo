package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SITECHECK_CONFIG_DIR", "")
	newDir := filepath.Join(home, ".config", "sitecheck")
	legacy := filepath.Join(home, ".config", "commandcenter")

	if got := ConfigDir(); got != newDir {
		t.Fatalf("fresh install: got %s, want %s", got, newDir)
	}
	os.MkdirAll(legacy, 0o700)
	os.WriteFile(filepath.Join(legacy, "sitecheck.env"), []byte("X=1\n"), 0o600)
	if got := ConfigDir(); got != legacy {
		t.Fatalf("legacy only: got %s, want %s", got, legacy)
	}
	os.MkdirAll(newDir, 0o700)
	if got := ConfigDir(); got != newDir {
		t.Fatalf("both exist: got %s, want %s", got, newDir)
	}
	t.Setenv("SITECHECK_CONFIG_DIR", "/elsewhere")
	if got := ConfigDir(); got != "/elsewhere" {
		t.Fatalf("override: got %s", got)
	}
}
