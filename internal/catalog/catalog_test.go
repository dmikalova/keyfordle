package catalog_test

import (
	"encoding/json"
	"slices"
	"strconv"
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

// every is a checkout with a card of every kind the generator meets: pool
// cards and their reprints, a shared collector number, and every exclusion,
// reprinted where a reprint can happen.
var every = map[string]string{
	"callofthearchons": `[
		{"number": "001", "name": "Zed", "house": "staralliance", "type": "creature", "rarity": "Rare", "power": 3},
		{"number": "002", "name": "Alaka's Brew", "printed": "Alaka’s Brew", "house": "brobnar", "type": "action", "rarity": "Common", "amber": 1},
		{"number": "003", "name": "Gauntlet", "house": "brobnar", "type": "upgrade", "rarity": "Common"},
		{"number": "004", "name": "Lamp", "house": "logos", "type": "artifact", "rarity": "Uncommon"}
	]`,
	"massmutation": `[
		{"number": "125", "name": "Ultra Gravitron", "house": "logos", "type": "gigantic creature base", "rarity": "Rare", "power": 15},
		{"number": "126", "name": "Ultra Gravitron", "house": "logos", "type": "gigantic creature art", "rarity": "Rare"}
	]`,
	"darktidings": `[
		{"number": "001", "name": "Zed", "house": "logos", "type": "evil twin creature", "rarity": "Evil Twin"},
		{"number": "002", "name": "Grumpus", "house": "dis", "type": "token creature", "rarity": "Token"},
		{"number": "003", "name": "The Tide", "house": "thetide", "type": "the tide", "rarity": "The Tide"}
	]`,
	"menagerie": `[
		{"number": "200", "name": "Keyraken Tentacle", "house": "keyraken", "type": "creature", "rarity": "Rare"},
		{"number": "305", "name": "Veylan Analyst", "house": "logos", "type": "creature", "rarity": "Common"},
		{"number": "305", "name": "It's Coming...", "printed": "It’s Coming...", "house": "saurian", "type": "action", "rarity": "Rare"},
		{"number": "323", "name": "Ultra Gravitron", "house": "logos", "type": "gigantic creature base", "rarity": "Rare", "power": 15}
	]`,
	"tokensofchange": `[
		{"number": "010", "name": "Grumpus", "house": "dis", "type": "token creature", "rarity": "Token"}
	]`,
	"aemberskies": `[
		{"number": "S01", "name": "Archon Power", "house": "archonpower", "type": "archon power", "rarity": "Special"}
	]`,
	"propheticvisions": `[
		{"number": "P01", "name": "Prophecy", "house": "prophecy", "type": "prophecy", "rarity": "Special"}
	]`,
	"vaultmasters2026": `[
		{"number": "100", "name": "Zed", "house": "dis", "type": "creature", "rarity": "Common", "power": 9}
	]`,
}

// printings lists every catalog entry in a checkout as <set-slug>/<number>.
func printings(t *testing.T, catalogs map[string]string) []string {
	t.Helper()

	var out []string

	for slug, body := range catalogs {
		var srcs []struct {
			Number string `json:"number"`
		}

		if err := json.Unmarshal([]byte(body), &srcs); err != nil {
			t.Fatal(err)
		}

		for _, s := range srcs {
			out = append(out, slug+"/"+s.Number)
		}
	}

	slices.Sort(out)

	return out
}

func TestBuildCoversEveryCatalogEntryOnce(t *testing.T) {
	res, err := catalog.Build(vex(every))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var covered []string

	for _, e := range res.Pool {
		ps, ok := res.PoolPrintings[e.Name]
		if !ok || len(ps) == 0 {
			t.Errorf("pool card %q covers no catalog entry", e.Name)
		}

		covered = append(covered, ps...)
	}

	if len(res.PoolPrintings) != len(res.Pool) {
		t.Errorf("PoolPrintings has %d cards, the pool %d", len(res.PoolPrintings), len(res.Pool))
	}

	for _, e := range res.Excluded {
		if len(e.Printings) == 0 {
			t.Errorf("excluded card %q covers no catalog entry", e.Name)
		}

		covered = append(covered, e.Printings...)
	}

	slices.Sort(covered)

	if want := printings(t, every); !slices.Equal(covered, want) {
		t.Fatalf("covered entries\n%v\nwant each catalog entry once\n%v", covered, want)
	}

	if res.Count() != len(covered) {
		t.Errorf("Count = %d, want %d", res.Count(), len(covered))
	}
}

func TestBuildPoolHoldsDeckAndReferenceCards(t *testing.T) {
	res, err := catalog.Build(vex(every))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var got []string

	for _, e := range res.Pool {
		got = append(got, strings.Join(
			[]string{e.Name, e.CardType, e.House, e.Rarity, strconv.FormatBool(e.Reference)},
			" | ",
		))
	}

	// A reference card has no house and is Special, the tide included. The
	// Keyraken adventure's cards are deck cards of the Keyraken house.
	want := []string{
		"Alaka’s Brew | Action | Brobnar | Common | false",
		"Archon Power | Archon Power |  | Special | true",
		"Gauntlet | Upgrade | Brobnar | Common | false",
		"It’s Coming... | Action | Saurian | Rare | false",
		"Keyraken Tentacle | Creature | Keyraken | Rare | false",
		"Lamp | Artifact | Logos | Uncommon | false",
		"Prophecy | Prophecy |  | Special | true",
		"The Tide | The Tide |  | Special | true",
		"Veylan Analyst | Creature | Logos | Common | false",
		"Zed | Creature | Star Alliance | Rare | false",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("pool\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestBuildRecordsEveryExclusion(t *testing.T) {
	res, err := catalog.Build(vex(every))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var got []string

	for _, e := range res.Excluded {
		got = append(got, strings.Join(
			[]string{
				string(e.Reason),
				e.Name,
				e.Set,
				e.Number,
				e.Type,
				e.House,
				strings.Join(e.Printings, ","),
			},
			" | ",
		))
	}

	// Sorted by reason in document order, then name. A gigantic's halves and
	// reprints are one card; so are a token's reprints.
	want := []string{
		"gigantic | Ultra Gravitron | massmutation | 125 | gigantic creature base | logos | massmutation/125,massmutation/126,menagerie/323",
		"evil-twin | Zed | darktidings | 001 | evil twin creature | logos | darktidings/001",
		"token-creature | Grumpus | darktidings | 002 | token creature | dis | darktidings/002,tokensofchange/010",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("excluded\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestBuildConvertsAndDedupes(t *testing.T) {
	res, err := catalog.Build(vex(every))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	byName := map[string]int{}
	for i, e := range res.Pool {
		byName[e.Name] = i
	}

	brew, zed := res.Pool[byName["Alaka’s Brew"]], res.Pool[byName["Zed"]]

	if brew.Folded != "Alaka's Brew" {
		t.Errorf("brew folded name = %q", brew.Folded)
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

	if got := res.PoolPrintings["Zed"]; !slices.Equal(
		got,
		[]string{"callofthearchons/001", "vaultmasters2026/100"},
	) {
		t.Errorf("zed printings = %v", got)
	}
}

func TestBuildNamesImagesOnASharedNumber(t *testing.T) {
	// Menagerie #305 is two cards. The token on #306 shares its number with a
	// pool card, so an excluded card still counts toward a shared number.
	fsys := vex(map[string]string{
		"menagerie": `[
			{"number": "304", "name": "Lone Card", "house": "dis", "type": "action", "rarity": "Common"},
			{"number": "305", "name": "Veylan Analyst", "house": "logos", "type": "creature", "rarity": "Common"},
			{"number": "305", "name": "It's Coming...", "printed": "It’s Coming...", "house": "saurian", "type": "action", "rarity": "Rare"},
			{"number": "306", "name": "\"Big\" Bot 2.0", "house": "logos", "type": "creature", "rarity": "Rare"},
			{"number": "306", "name": "Bot Token", "house": "logos", "type": "token creature", "rarity": "Token"}
		]`,
	})

	res, err := catalog.Build(fsys)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	want := map[string]string{
		"Lone Card":       "menagerie/304",
		"Veylan Analyst":  "menagerie/305-veylan-analyst",
		"It’s Coming...":  "menagerie/305-its-coming",
		"\"Big\" Bot 2.0": "menagerie/306-big-bot-2-0",
	}

	if len(res.Pool) != len(want) {
		t.Fatalf("Build returned %d pool cards, want %d", len(res.Pool), len(want))
	}

	for _, e := range res.Pool {
		if got, wantURL := e.Image, catalog.ImageBaseURL+want[e.Name]+catalog.ImageExt; got != wantURL {
			t.Errorf("%s image = %q, want %q", e.Name, got, wantURL)
		}
	}
}

func TestBuildFailsLoudly(t *testing.T) {
	tests := map[string]string{
		"unknown type":   `[{"number": "1", "name": "X", "house": "dis", "type": "location", "rarity": "Rare"}]`,
		"unknown house":  `[{"number": "1", "name": "X", "house": "mystery", "type": "action", "rarity": "Rare"}]`,
		"unknown rarity": `[{"number": "1", "name": "X", "house": "dis", "type": "action", "rarity": "Mythic"}]`,
		"one name, two types": `[
			{"number": "1", "name": "X", "house": "dis", "type": "action", "rarity": "Rare"},
			{"number": "2", "name": "X", "house": "archonpower", "type": "archon power", "rarity": "Special"}
		]`,
		"bad json":      `{`,
		"no pool cards": `[]`,
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

func TestExcludedDocCountsEveryReason(t *testing.T) {
	res, err := catalog.Build(vex(every))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	doc := string(catalog.ExcludedDoc(res, "excluded-cards.json"))

	for _, r := range catalog.Reasons {
		if !strings.Contains(doc, "| `"+string(r)+"` | 1 |") {
			t.Errorf("the page does not count reason %s:\n%s", r, doc)
		}

		if !strings.Contains(doc, "Reason: `"+string(r)+"`. Cards: 1.") {
			t.Errorf("the page has no section for reason %s", r)
		}
	}
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
