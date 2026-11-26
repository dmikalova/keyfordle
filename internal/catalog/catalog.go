// Package catalog builds KeyFordle's card pool from the keyforge-data dataset
// (BuildWithDataset). Until the dataset has every set, it reads vex's set
// catalogs for the other sets, as described below.
//
// vex keeps one JSON catalog per set at internal/cards/provenance/<slug>.json,
// built from the Master Vault decks feed. Build folds them into the pool that
// internal/cards embeds: one card per name, from the first set in release order
// that printed it, in the value formats the lifted React page compares. Every
// card it leaves out is recorded, with the reason, so it can be added later.
//
// The pool holds the deck cards of every set, the Keyraken adventure's cards
// included, and the reference cards that sit beside a deck: archon powers,
// prophecies and the tide. A reference card has no house. internal/cards picks
// one as the card of the day on about 2% of days.
package catalog

import (
	"cmp"
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

// The card images live under images/cards/ in the public keyforge.cards
// bucket, one per <set-slug>/<file>. The extension is a constant of its own
// because it follows the upload's format: WebP at quality 85 (`mage images`).
const (
	ImageBaseURL = "https://storage.googleapis.com/mklv-keyforge-cards-public/images/cards/"
	ImageExt     = ".webp"
)

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
	"keyraken":     "Keyraken",
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

// cardTypes maps the deck card types the pool holds to the type the page shows.
var cardTypes = map[string]string{
	"action":   "Action",
	"artifact": "Artifact",
	"creature": "Creature",
	"upgrade":  "Upgrade",
}

// referenceTypes maps the reference card types the pool holds to the type the
// page shows. A reference card has no house, and its rarity is Special: the
// tide's catalog rarity, "The Tide", is not one the page orders.
var referenceTypes = map[string]string{
	"archon power": "Archon Power",
	"prophecy":     "Prophecy",
	"the tide":     "The Tide",
}

// Reason is why a card is left out of the pool.
type Reason string

// The reasons, one per kind of card the pool leaves out. Reasons lists them in
// the order the excluded list and its document use.
const (
	ReasonGigantic Reason = "gigantic"
	ReasonEvilTwin Reason = "evil-twin"
	ReasonToken    Reason = "token-creature"
)

// Reasons lists every reason in document order.
var Reasons = []Reason{
	ReasonGigantic,
	ReasonEvilTwin,
	ReasonToken,
}

// excludedTypes are the card types the pool leaves out.
var excludedTypes = map[string]Reason{
	"evil twin creature":     ReasonEvilTwin,
	"gigantic creature art":  ReasonGigantic,
	"gigantic creature base": ReasonGigantic,
	"token creature":         ReasonToken,
}

// rarities are the rarities the page orders. Any other rarity on a pool card is
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

// printedName is the name as the card prints it.
func (s source) printedName() string {
	if s.Printed != "" {
		return s.Printed
	}

	return s.Name
}

// Excluded is one card the pool leaves out. Set, Number, Type and House are its
// first printing, in vex's spelling. Printings lists every catalog entry the
// card covers as <set-slug>/<number>, reprints and the art half of a gigantic
// included.
type Excluded struct {
	Name      string   `json:"name"`
	Set       string   `json:"set"`
	Number    string   `json:"number"`
	Type      string   `json:"type"`
	House     string   `json:"house"`
	Reason    Reason   `json:"reason"`
	Printings []string `json:"printings"`
}

// Result is what Build makes of the catalogs. Every catalog entry is covered
// exactly once: by a pool card, listed in PoolPrintings, or by an excluded
// card, listed in its Printings.
type Result struct {
	// Pool is the guessable cards, sorted by name.
	Pool []cards.Entry
	// PoolPrintings maps each pool card's name to the catalog entries it
	// covers, as <set-slug>/<number>: its first printing, then its reprints.
	PoolPrintings map[string][]string
	// Excluded is every card left out, sorted by reason, then name.
	Excluded []Excluded
}

// Count is the number of catalog entries the result covers.
func (r Result) Count() int {
	n := 0
	for _, ps := range r.PoolPrintings {
		n += len(ps)
	}

	for _, e := range r.Excluded {
		n += len(e.Printings)
	}

	return n
}

// Build reads every set's catalog from fsys, which is rooted at a vex checkout,
// and sorts each catalog entry into the pool or the excluded list.
func Build(fsys fs.FS) (Result, error) {
	return buildSets(fsys, Sets)
}

// buildSets is Build for the given sets only.
func buildSets(fsys fs.FS, sets []Set) (Result, error) {
	res := Result{PoolPrintings: map[string][]string{}}
	poolIndex := map[string]int{}     // folded name -> index in res.Pool
	excludedIndex := map[string]int{} // reason + folded name -> index in res.Excluded
	read := 0

	for _, set := range sets {
		srcs, err := readCatalog(fsys, set)
		if err != nil {
			return Result{}, err
		}

		read += len(srcs)

		perNumber := map[string]int{}
		for _, src := range srcs {
			perNumber[src.Number]++
		}

		for _, src := range srcs {
			printing := set.Slug + "/" + src.Number

			if reason, ok := exclusion(src); ok {
				key := string(reason) + "\n" + strings.ToLower(src.Name)
				if i, seen := excludedIndex[key]; seen {
					res.Excluded[i].Printings = append(res.Excluded[i].Printings, printing)

					continue
				}

				excludedIndex[key] = len(res.Excluded)
				res.Excluded = append(res.Excluded, Excluded{
					Name:      src.printedName(),
					Set:       set.Slug,
					Number:    src.Number,
					Type:      src.Type,
					House:     src.House,
					Reason:    reason,
					Printings: []string{printing},
				})

				continue
			}

			// Every printing is converted, reprints included, so a bad value
			// anywhere in a catalog is reported.
			entry, err := convert(set, src, perNumber[src.Number] > 1)
			if err != nil {
				return Result{}, fmt.Errorf("%s %s: %w", set.Slug, src.Number, err)
			}

			// A reprint keeps the set that first printed the card. Two cards of
			// different types with one name are two cards, and the pool cannot
			// tell them apart.
			key := strings.ToLower(src.Name)
			if i, seen := poolIndex[key]; seen {
				first := res.Pool[i]
				if first.CardType != entry.CardType {
					return Result{}, fmt.Errorf(
						"%s %s: %q is a %s and a %s",
						set.Slug, src.Number, entry.Name, first.CardType, entry.CardType,
					)
				}

				res.PoolPrintings[first.Name] = append(res.PoolPrintings[first.Name], printing)

				continue
			}

			poolIndex[key] = len(res.Pool)
			res.PoolPrintings[entry.Name] = []string{printing}
			res.Pool = append(res.Pool, entry)
		}
	}

	if len(res.Pool) == 0 {
		return Result{}, errors.New("the catalogs hold no pool cards")
	}

	if len(res.PoolPrintings) != len(res.Pool) {
		return Result{}, errors.New("two pool cards print the same name")
	}

	if got := res.Count(); got != read {
		return Result{}, fmt.Errorf("the result covers %d catalog entries, want %d", got, read)
	}

	slices.SortFunc(res.Pool, func(a, b cards.Entry) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(res.Excluded, func(a, b Excluded) int {
		return cmp.Or(
			cmp.Compare(slices.Index(Reasons, a.Reason), slices.Index(Reasons, b.Reason)),
			strings.Compare(a.Name, b.Name),
		)
	})

	return res, nil
}

// readCatalog reads one set's catalog.
func readCatalog(fsys fs.FS, set Set) ([]source, error) {
	raw, err := fs.ReadFile(fsys, path.Join(CatalogDir, set.Slug+".json"))
	if err != nil {
		return nil, fmt.Errorf("read the %s catalog: %w", set.Slug, err)
	}

	var srcs []source
	if err := json.Unmarshal(raw, &srcs); err != nil {
		return nil, fmt.Errorf("parse the %s catalog: %w", set.Slug, err)
	}

	return srcs, nil
}

// exclusion returns why the pool leaves src out, if it does.
func exclusion(src source) (Reason, bool) {
	reason, ok := excludedTypes[src.Type]

	return reason, ok
}

// convert folds one catalog card into a pool entry. sharedNumber is true when
// another card in the set's catalog has the same collector number.
func convert(set Set, src source, sharedNumber bool) (cards.Entry, error) {
	if refType, ok := referenceTypes[src.Type]; ok {
		return entryOf(set, src, sharedNumber, refType, "", "Special", true), nil
	}

	cardType, ok := cardTypes[src.Type]
	if !ok {
		return cards.Entry{}, fmt.Errorf("unknown card type %q", src.Type)
	}

	house, ok := houses[src.House]
	if !ok {
		return cards.Entry{}, fmt.Errorf("unknown house %q", src.House)
	}

	if !rarities[src.Rarity] {
		return cards.Entry{}, fmt.Errorf("unknown rarity %q", src.Rarity)
	}

	return entryOf(set, src, sharedNumber, cardType, house, src.Rarity, false), nil
}

// entryOf builds the pool entry of src from its converted type, house and
// rarity.
func entryOf(
	set Set,
	src source,
	sharedNumber bool,
	cardType, house, rarity string,
	reference bool,
) cards.Entry {
	name := src.printedName()

	c := cards.Card{
		Name:       name,
		House:      house,
		CardType:   cardType,
		Rarity:     rarity,
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

	entry := cards.Entry{Card: c, Reference: reference}
	if src.Name != name {
		entry.Folded = src.Name
	}

	return entry
}

// imageName is the card's file name in the image bucket, as vex's importer
// writes it: <number><ext>, or <number>-<name slug><ext> when another card in
// the same set shares the number (Menagerie #305 is Veylan Analyst and It's
// Coming...).
func imageName(src source, sharedNumber bool) string {
	if !sharedNumber {
		return src.Number + ImageExt
	}

	return src.Number + "-" + slug(src.Name) + ImageExt
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
