// Package env loads settings from a private config file outside the repo.
package env

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ConfigDir is where settings and secrets live, outside any repo:
// $SITECHECK_CONFIG_DIR, else ~/.config/sitecheck. For installs that predate
// this repo, ~/.config/commandcenter is used when it holds sitecheck.env and
// the new folder doesn't exist yet.
func ConfigDir() string {
	if d := os.Getenv("SITECHECK_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "sitecheck")
	legacy := filepath.Join(home, ".config", "commandcenter")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(legacy, "sitecheck.env")); err == nil {
			return legacy
		}
	}
	return dir
}

// Load reads KEY=VALUE lines from <ConfigDir>/<name>.env. Values already set
// in the environment win. A missing file is fine. Returns the path it tried.
func Load(name string) string {
	path := filepath.Join(ConfigDir(), name+".env")
	LoadFile(path)
	return path
}

// LoadFile reads KEY=VALUE lines (optionally "export KEY=VALUE", # comments).
func LoadFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if key != "" && os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
