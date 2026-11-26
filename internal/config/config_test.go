package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dmikalova/keyfordle/internal/config"
)

func write(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestLoad(t *testing.T) {
	t.Setenv("PORT", "")

	cfg, err := config.Load(
		write(
			t,
			`{"DATABASE_URL_TRANSACTION": "postgres://x", "DATABASE_URL_SESSION": "postgres://y"}`,
		),
	)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DatabaseURL != "postgres://x" || cfg.Port != "8080" {
		t.Fatalf("cfg = %+v", cfg)
	}

	t.Setenv("PORT", "9000")

	cfg, err = config.Load(write(t, `{"DATABASE_URL_TRANSACTION": "postgres://x"}`))
	if err != nil || cfg.Port != "9000" {
		t.Fatalf("cfg = %+v, err %v", cfg, err)
	}
}

func TestLoadRejects(t *testing.T) {
	tests := map[string]string{
		"no database URL": `{}`,
		"bad json":        `{`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := config.Load(write(t, body)); err == nil {
				t.Fatal("Load accepted the file")
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		if _, err := config.Load(filepath.Join(t.TempDir(), "absent.json")); err == nil {
			t.Fatal("Load accepted a missing file")
		}
	})
}
