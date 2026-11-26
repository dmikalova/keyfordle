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

// The files Cards writes. Only the pool is embedded in the binary; the
// excluded list and its page are a record for people.
const (
	poolPath        = "internal/cards/cards.json"
	excludedPath    = "docs/excluded-cards.json"
	excludedDocPath = "docs/excluded-cards.md"
)

// Cards regenerates the card pool from vex's catalogs. It reads every set
// catalog under internal/cards/provenance in a vex checkout and writes the
// pool to internal/cards/cards.json, and the cards it leaves out to
// docs/excluded-cards.json and docs/excluded-cards.md. Pass -vex=<dir> for the
// checkout; it defaults to ../vex. Commit the results.
func Cards(vex *string) error {
	dir := "../vex"
	if vex != nil && *vex != "" {
		dir = *vex
	}

	res, err := catalog.Build(os.DirFS(dir))
	if err != nil {
		return err
	}

	if err := writeJSON(poolPath, res.Pool); err != nil {
		return err
	}

	if err := writeJSON(excludedPath, res.Excluded); err != nil {
		return err
	}

	if err := os.WriteFile(
		excludedDocPath,
		catalog.ExcludedDoc(res, "excluded-cards.json"),
		0o600,
	); err != nil {
		return fmt.Errorf("write %s: %w", excludedDocPath, err)
	}

	// vp check formats JSON and Markdown too, so the files are written in its
	// layout. Run `vp install` first if this fails.
	if err := sh.Run(
		"pnpm",
		"exec",
		"vp",
		"fmt",
		poolPath,
		excludedPath,
		excludedDocPath,
	); err != nil {
		return fmt.Errorf("format the card files: %w", err)
	}

	fmt.Printf("wrote %d pool cards to %s\n", len(res.Pool), poolPath)
	fmt.Printf("wrote %d excluded cards to %s\n", len(res.Excluded), excludedPath)

	return nil
}

// writeJSON writes v as indented JSON, with &, < and > left as they are.
func writeJSON(path string, v any) error {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}
