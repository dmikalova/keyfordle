//go:build mage

// keyfordle developer tasks. The ci namespace comes from project-standards'
// shared ci package. Run `mage -l` to list them, and
// `mage ci:fix && mage ci:check` before calling work done.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	// mage:import ci
	_ "github.com/dmikalova/project-standards/ci"
	"github.com/magefile/mage/sh"

	"github.com/dmikalova/keyfordle/internal/catalog"
)

// poolPath is the card pool internal/cards embeds.
const poolPath = "internal/cards/cards.json"

// Cards regenerates the card pool from vex's catalogs. It reads every set
// catalog under internal/cards/provenance in a vex checkout and writes
// internal/cards/cards.json. Pass -vex=<dir> for the checkout; it defaults to
// ../vex. Commit the result: the binary embeds it.
func Cards(vex *string) error {
	dir := "../vex"
	if vex != nil && *vex != "" {
		dir = *vex
	}

	entries, err := catalog.Build(os.DirFS(dir))
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	if err := enc.Encode(entries); err != nil {
		return fmt.Errorf("encode the pool: %w", err)
	}

	if err := os.WriteFile(poolPath, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", poolPath, err)
	}

	// vp check formats JSON too, so the pool is written in its layout. Run
	// `vp install` first if this fails.
	if err := sh.Run("pnpm", "exec", "vp", "fmt", poolPath); err != nil {
		return fmt.Errorf("format %s: %w", poolPath, err)
	}

	fmt.Printf("wrote %d cards to %s\n", len(entries), poolPath)

	return nil
}
