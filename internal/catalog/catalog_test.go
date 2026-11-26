package catalog_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dmikalova/keyfordle/internal/catalog"
)

// vex returns a vex checkout holding the given catalogs and an empty catalog for
// every other set.
func vex(catalogs map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}

	for _, set := range catalog.Sets {
		body, ok := catalogs[set.Slug]
		if !ok {
			body = "[]"
		}

		fsys[catalog.CatalogDir+"/"+set.Slug+".json"] = &fstest.MapFile{Data: []byte(body)}
	}

	return fsys
}

func TestBuildConvertsAndDedupes(t *testing.T) {
	fsys := vex(map[string]string{
		"callofthearchons": `[
			{"number": "001", "name": "Zed", "house": "staralliance", "type": "creature", "rarity": "Rare", "power": 3},
			{"number": "002", "name": "Alaka's Brew", "printed": "Alaka’s Brew", "house": "brobnar", "type": "action", "rarity": "Common", "amber": 1}
		]`,
		"darktidings": `[
			{"number": "001", "name": "Zed", "house": "logos", "type": "evil twin creature", "rarity": "Evil Twin"},
			{"number": "002", "name": "Token", "house": "logos", "type": "token creature", "rarity": "Token"}
		]`,
		"vaultmasters2026": `[
			{"number": "100", "name": "Zed", "house": "dis", "type": "creature", "rarity": "Common", "power": 9}
		]`,
		"menagerie": `[
			{"number": "200", "name": "Keyraken Tentacle", "house": "keyraken", "type": "creature", "rarity": "Rare"}
		]`,
	})

	entries, err := catalog.Build(fsys)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Build returned %d entries, want 2: %+v", len(entries), entries)
	}

	brew, zed := entries[0], entries[1]

	if brew.Name != "Alaka’s Brew" || brew.Folded != "Alaka's Brew" {
		t.Errorf("brew name = %q folded %q", brew.Name, brew.Folded)
	}

	if brew.Power != nil || brew.Armor != nil {
		t.Errorf("an action has power %v armor %v, want none", brew.Power, brew.Armor)
	}

	if brew.Traits == nil {
		t.Error("traits are null, want []")
	}

	// The first set wins: Call of the Archons, not the evil twin or the reprint.
	if zed.House != "Star Alliance" || zed.Expansion != 341 || zed.Rarity != "Rare" {
		t.Errorf("zed = %+v, want the Call of the Archons printing", zed.Card)
	}

	if zed.CardType != "Creature" || *zed.Power != "3" || *zed.Armor != "0" {
		t.Errorf("zed type %q power %v armor %v", zed.CardType, zed.Power, zed.Armor)
	}

	if !strings.HasSuffix(zed.Image, "/cards/callofthearchons/001.png") {
		t.Errorf("zed image = %q", zed.Image)
	}
}

func TestBuildShowsAGiganticBaseAsACreature(t *testing.T) {
	fsys := vex(map[string]string{
		"draconianmeasures": `[
			{"number": "047", "name": "Gigantor", "house": "geistoid", "type": "gigantic creature base", "rarity": "Special", "power": 8},
			{"number": "048", "name": "Gigantor Art", "house": "geistoid", "type": "gigantic creature art", "rarity": "Special"}
		]`,
	})

	entries, err := catalog.Build(fsys)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(entries) != 1 || entries[0].CardType != "Creature" || *entries[0].Power != "8" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestBuildNamesImagesOnASharedNumber(t *testing.T) {
	// Menagerie #305 is two cards. The token on #306 shares its number with an
	// art card, so a skipped card still counts toward a shared number.
	fsys := vex(map[string]string{
		"menagerie": `[
			{"number": "304", "name": "Lone Card", "house": "dis", "type": "action", "rarity": "Common"},
			{"number": "305", "name": "Veylan Analyst", "house": "logos", "type": "creature", "rarity": "Common"},
			{"number": "305", "name": "It's Coming...", "printed": "It’s Coming...", "house": "saurian", "type": "action", "rarity": "Rare"},
			{"number": "306", "name": "\"Big\" Bot 2.0", "house": "logos", "type": "creature", "rarity": "Rare"},
			{"number": "306", "name": "Bot Token", "house": "logos", "type": "token creature", "rarity": "Token"}
		]`,
	})

	entries, err := catalog.Build(fsys)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	want := map[string]string{
		"Lone Card":       "menagerie/304.png",
		"Veylan Analyst":  "menagerie/305-veylan-analyst.png",
		"It’s Coming...":  "menagerie/305-its-coming.png",
		"\"Big\" Bot 2.0": "menagerie/306-big-bot-2-0.png",
	}

	if len(entries) != len(want) {
		t.Fatalf("Build returned %d entries, want %d", len(entries), len(want))
	}

	for _, e := range entries {
		if got := strings.TrimPrefix(e.Image, catalog.ImageBaseURL); got != want[e.Name] {
			t.Errorf("%s image = %q, want %q", e.Name, got, want[e.Name])
		}
	}
}

func TestBuildFailsLoudly(t *testing.T) {
	tests := map[string]string{
		"unknown type":   `[{"number": "1", "name": "X", "house": "dis", "type": "location", "rarity": "Rare"}]`,
		"unknown house":  `[{"number": "1", "name": "X", "house": "mystery", "type": "action", "rarity": "Rare"}]`,
		"unknown rarity": `[{"number": "1", "name": "X", "house": "dis", "type": "action", "rarity": "Mythic"}]`,
		"bad json":       `{`,
		"no deck cards":  `[]`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := catalog.Build(
				vex(map[string]string{"callofthearchons": body}),
			); err == nil {
				t.Fatal("Build accepted the catalog")
			}
		})
	}

	t.Run("missing catalog", func(t *testing.T) {
		if _, err := catalog.Build(fstest.MapFS{}); err == nil {
			t.Fatal("Build accepted a checkout with no catalogs")
		}
	})
}

func TestSetsAreInExpansionOrder(t *testing.T) {
	for i := 1; i < len(catalog.Sets); i++ {
		if catalog.Sets[i-1].Expansion >= catalog.Sets[i].Expansion {
			t.Fatalf("%s (%d) is not after %s (%d)",
				catalog.Sets[i].Slug, catalog.Sets[i].Expansion,
				catalog.Sets[i-1].Slug, catalog.Sets[i-1].Expansion)
		}
	}
}
