//go:build mage

// keyfordle developer tasks. The ci namespace comes from project-standards'
// shared ci package. Run `mage -l` to list them, and
// `mage ci:fix && mage ci:check` before calling work done.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/dmikalova/keyforge-data/card"
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

// Cards regenerates the card pool from the keyforge-data dataset. It reads
// vex's set catalogs (internal/cards/provenance in a vex checkout) only for the
// sets that the dataset does not have yet. It writes the pool to
// internal/cards/cards.json, and the cards it leaves out to
// docs/excluded-cards.json and docs/excluded-cards.md. Pass -vex=<dir> for the
// checkout; it defaults to ../vex. Set DATASET_URL to read another dataset.
// Commit the results.
func Cards(vex *string) error {
	dir := "../vex"
	if vex != nil && *vex != "" {
		dir = *vex
	}

	ds, err := loadDataset()
	if err != nil {
		return err
	}

	res, err := catalog.BuildWithDataset(ds, os.DirFS(dir))
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

// loadDataset downloads the keyforge-data dataset. It reads the pointer file
// (catalog.LatestURL) and then the version file that the pointer names.
// DATASET_URL, when set, is a dataset file to read instead, for local work.
func loadDataset() (*card.Dataset, error) {
	client := &http.Client{Timeout: time.Minute}

	url := os.Getenv("DATASET_URL")
	if url == "" {
		var latest card.Latest
		if err := getJSON(client, catalog.LatestURL, &latest); err != nil {
			return nil, fmt.Errorf("the dataset pointer: %w", err)
		}

		if latest.URL == "" {
			return nil, fmt.Errorf("the dataset pointer %s has no URL", catalog.LatestURL)
		}

		url = latest.URL
	}

	var ds card.Dataset
	if err := getJSON(client, url, &ds); err != nil {
		return nil, fmt.Errorf("the dataset: %w", err)
	}

	fmt.Printf("dataset %s: %d cards in %d sets\n", ds.Version, len(ds.Cards), len(ds.Sets))

	return &ds, nil
}

// getJSON reads a JSON file into v.
func getJSON(client *http.Client, url string, v any) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get %s: %s", url, resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("parse %s: %w", url, err)
	}

	return nil
}
