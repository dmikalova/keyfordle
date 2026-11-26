// Package config loads keyfordle's runtime settings.
//
// Cloud Run delivers every secret as one JSON object mounted at
// /secrets/config.json by the cloud-run-app module. Nothing sensitive arrives
// through the environment.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultSecretsPath is where the cloud-run-app module mounts the config secret.
const DefaultSecretsPath = "/secrets/config.json"

// Config is the fully resolved runtime configuration.
type Config struct {
	DatabaseURL string
	Port        string
}

// secrets mirrors the JSON written by the cloud-run-app module's secrets map.
// DATABASE_URL_SESSION is also in there; CI uses it to apply db/schema.hcl.
type secrets struct {
	DatabaseURLTransaction string `json:"DATABASE_URL_TRANSACTION"`
}

// Load reads the mounted secret and the environment, then validates both.
func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultSecretsPath
	}

	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return Config{}, fmt.Errorf("open the secrets directory of %s: %w", path, err)
	}
	defer func() { _ = root.Close() }()

	raw, err := root.ReadFile(filepath.Base(path))
	if err != nil {
		return Config{}, fmt.Errorf("read the secrets file %s: %w", path, err)
	}

	var s secrets
	if err := json.Unmarshal(raw, &s); err != nil {
		return Config{}, fmt.Errorf("parse the secrets file %s: %w", path, err)
	}

	cfg := Config{
		DatabaseURL: s.DatabaseURLTransaction,
		Port:        os.Getenv("PORT"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return cfg, cfg.validate()
}

func (c Config) validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("DATABASE_URL_TRANSACTION is empty")
	}

	return nil
}
