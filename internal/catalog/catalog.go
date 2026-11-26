// Package catalog builds Keyfordle's card pool from vex's set catalogs.
//
// vex keeps one JSON catalog per set at internal/cards/provenance/<slug>.json,
// built from the Master Vault decks feed. Build folds them into the pool that
// internal/cards embeds: one card per name, from the first set in release order
// that printed it, in the value formats the lifted React page compares.
package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/dmikalova/keyfordle/internal/cards"
)

// ImageBaseURL is where the card images live: the public keyforge.cards image
// bucket, keyed by set slug and collector number.
const ImageBaseURL = "https://storage.googleapis.com/mklv-keyforge-cards-images/cards/"

// CatalogDir is where vex keeps its set catalogs, relative to a vex checkout.
const CatalogDir = "internal/cards/provenance"

// Set is one KeyForge set: vex's slug and the Master Vault expansion id that
// the page's EXPANSIONS map keys on.
type Set struct {
	Slug      string
	Expansion int
}

// Sets lists every set in release order, which is also ascending expansion id
// order, so the page's earlier/later arrows hold. The ids are copied from
// importSets in vex's magefiles/cardlookup/importprovenance.go.
var Sets = []Set{
	{"callofthearchons", 341},
	{"ageofascension", 435},
	{"worldscollide", 452},
	{"anomalyexpansion", 453},
	{"massmutation", 479},
	{"darktidings", 496},
	{"windsofexchange", 600},
	{"unchained2022", 601},
	{"vaultmasters2023", 609},
	{"grimreminders", 700},
	{"menagerie", 722},
	{"vaultmasters2024", 737},
	{"aemberskies", 800},
	{"tokensofchange", 855},
	{"moremutation", 874},
	{"propheticvisions", 886},
	{"martiancivilwar", 892},
	{"discovery", 907},
	{"crucibleclash", 918},
	{"draconianmeasures", 928},
	{"vaultmasters2025", 939},
	{"vaultmasters2026", 964},
}

// houses maps vex's house slugs to the names Master Vault prints.
var houses = map[string]string{
	"brobnar":      "Brobnar",
	"dis":          "Dis",
	"ekwidon":      "Ekwidon",
	"elders":       "Elders",
	"geistoid":     "Geistoid",
	"ironyxrebels": "Ironyx Rebels",
	"logos":        "Logos",
	"mars":         "Mars",
	"ouboros":      "Ouboros",
	"redemption":   "Redemption",
	"sanctum":      "Sanctum",
	"saurian":      "Saurian",
	"shadows":      "Shadows",
	"skyborn":      "Skyborn",
	"staralliance": "Star Alliance",
	"unfathomable": "Unfathomable",
	"untamed":      "Untamed",
}

// skippedHouses are houses no player deck holds. The Keyraken cards make up the
// adventure's boss deck.
var skippedHouses = map[string]bool{
	"keyraken": true,
}

// cardTypes maps vex's deck card types to the type the page shows. A gigantic
// creature's base half is the half that carries its stats, and it is a
// creature, so it shows as one.
var cardTypes = map[string]string{
	"action":                 "Action",
	"artifact":               "Artifact",
	"creature":               "Creature",
	"gigantic creature base": "Creature",
	"upgrade":                "Upgrade",
}

// skippedTypes are the card types that are not deck cards a player can guess.
// An evil twin shares its name with a regular creature that stays in the pool,
// and vex records no power or armor for it.
var skippedTypes = map[string]bool{
	"archon power":          true,
	"evil twin creature":    true,
	"gigantic creature art": true,
	"prophecy":              true,
	"the tide":              true,
	"token creature":        true,
}

// rarities are the rarities the page orders. Any other rarity on a deck card is
// an error, so a new one is noticed rather than compared as unknown.
var rarities = map[string]bool{
	"Common":   true,
	"Uncommon": true,
	"Rare":     true,
	"Special":  true,
	"Variant":  true,
}

// source is one card as vex's catalog stores it.
type source struct {
	Number  string   `json:"number"`
	Name    string   `json:"name"`
	Printed string   `json:"printed"`
	House   string   `json:"house"`
	Type    string   `json:"type"`
	Rarity  string   `json:"rarity"`
	Traits  []string `json:"traits"`
	Power   int      `json:"power"`
	Armor   int      `json:"armor"`
	Amber   int      `json:"amber"`
	Text    string   `json:"text"`
}

// Build reads every set's catalog from fsys, which is rooted at a vex checkout,
// and returns the pool entries sorted by name.
func Build(fsys fs.FS) ([]cards.Entry, error) {
	seen := map[string]bool{}

	var out []cards.Entry

	for _, set := range Sets {
		raw, err := fs.ReadFile(fsys, path.Join(CatalogDir, set.Slug+".json"))
		if err != nil {
			return nil, fmt.Errorf("read the %s catalog: %w", set.Slug, err)
		}

		var srcs []source
		if err := json.Unmarshal(raw, &srcs); err != nil {
			return nil, fmt.Errorf("parse the %s catalog: %w", set.Slug, err)
		}

		perNumber := map[string]int{}
		for _, src := range srcs {
			perNumber[src.Number]++
		}

		for _, src := range srcs {
			entry, keep, err := convert(set, src, perNumber[src.Number] > 1)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", set.Slug, src.Number, err)
			}

			// A reprint keeps the set that first printed the card.
			key := strings.ToLower(src.Name)
			if !keep || seen[key] {
				continue
			}

			seen[key] = true

			out = append(out, entry)
		}
	}

	if len(out) == 0 {
		return nil, errors.New("the catalogs hold no deck cards")
	}

	slices.SortFunc(out, func(a, b cards.Entry) int { return strings.Compare(a.Name, b.Name) })

	return out, nil
}

// convert folds one catalog card into a pool entry. keep is false for a card
// no player deck holds. sharedNumber is true when another card in the set's
// catalog has the same collector number.
func convert(set Set, src source, sharedNumber bool) (entry cards.Entry, keep bool, err error) {
	if skippedTypes[src.Type] || skippedHouses[src.House] {
		return cards.Entry{}, false, nil
	}

	cardType, ok := cardTypes[src.Type]
	if !ok {
		return cards.Entry{}, false, fmt.Errorf("unknown card type %q", src.Type)
	}

	house, ok := houses[src.House]
	if !ok {
		return cards.Entry{}, false, fmt.Errorf("unknown house %q", src.House)
	}

	if !rarities[src.Rarity] {
		return cards.Entry{}, false, fmt.Errorf("unknown rarity %q", src.Rarity)
	}

	name := src.Name
	if src.Printed != "" {
		name = src.Printed
	}

	c := cards.Card{
		Name:       name,
		House:      house,
		CardType:   cardType,
		Rarity:     src.Rarity,
		Amber:      src.Amber,
		Traits:     traits(src.Traits),
		CardText:   src.Text,
		CardNumber: src.Number,
		Expansion:  set.Expansion,
		Image:      ImageBaseURL + set.Slug + "/" + imageName(src, sharedNumber),
	}

	// A creature always has power and armor, zero included. Other types have
	// neither, which the page shows as a dash and compares as equal.
	if cardType == "Creature" {
		power := strconv.Itoa(src.Power)
		armor := strconv.Itoa(src.Armor)
		c.Power = &power
		c.Armor = &armor
	}

	entry = cards.Entry{Card: c}
	if src.Name != name {
		entry.Folded = src.Name
	}

	return entry, true, nil
}

// imageName is the card's file name in the image bucket, as vex's importer
// writes it: <number>.png, or <number>-<name slug>.png when another card in the
// same set shares the number (Menagerie #305 is Veylan Analyst and It's
// Coming...).
func imageName(src source, sharedNumber bool) string {
	if !sharedNumber {
		return src.Number + ".png"
	}

	return src.Number + "-" + slug(src.Name) + ".png"
}

// slug lowercases s and keeps its ASCII letters and digits. Apostrophes and
// quotes are dropped, every other run of characters becomes one hyphen, and
// the hyphens at the ends are trimmed: "It's Coming..." is "its-coming".
func slug(s string) string {
	var b strings.Builder

	gap := false

	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			if gap && b.Len() > 0 {
				b.WriteByte('-')
			}

			gap = false

			b.WriteRune(r)
		case strings.ContainsRune(`'"‘’“”`, r):
		default:
			gap = true
		}
	}

	return b.String()
}

// traits returns the card's traits, or an empty list rather than null.
func traits(ts []string) []string {
	if ts == nil {
		return []string{}
	}

	return ts
}
