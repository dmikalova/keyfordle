package catalog

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/derive"

	"github.com/dmikalova/keyfordle/internal/cards"
)

// DatasetURL is the keyforge-data dataset in the public bucket.
const DatasetURL = "https://storage.googleapis.com/mklv-keyforge-cards-public/data/cards.json"

// datasetTypes maps the dataset's deck card types to the type the page shows.
var datasetTypes = map[string]string{
	"action":   "Action",
	"artifact": "Artifact",
	"creature": "Creature",
	"upgrade":  "Upgrade",
}

// referenceGroups maps the group of a dataset reference card to the type the
// page shows.
var referenceGroups = map[string]string{
	"archonpower": "Archon Power",
	"prophecy":    "Prophecy",
	"thetide":     "The Tide",
}

// tideIcon is the dataset's token for the tide icon in card text.
const tideIcon = "{tide}"

// BuildWithDataset builds the pool from the keyforge-data dataset, and reads
// vex's catalogs (fsys) only for the sets that the dataset does not have yet.
// A card in both keeps the dataset's version. When the dataset has every set,
// fsys is not read.
//
// From the dataset, the pool takes the deck cards, the reference cards, the
// token creatures as creatures, and each gigantic creature as one card with
// its joined image. It leaves out the evil twins, because each has the name
// of a regular creature.
func BuildWithDataset(ds *card.Dataset, fsys fs.FS) (Result, error) {
	expansions := map[string]int{}
	for _, s := range ds.Sets {
		expansions[s.Slug] = s.Expansion
	}

	res := Result{PoolPrintings: map[string][]string{}}
	seen := map[string]bool{}
	// anomalyOnly holds the cards that the dataset has only as anomaly
	// printings. The set that prints the card for decks can be one that the
	// crawl has not read yet, so vex's card wins for these.
	anomalyOnly := map[string]bool{}

	for i := range ds.Cards {
		c := &ds.Cards[i]
		if !slices.ContainsFunc(c.Printings, func(p card.Printing) bool { return !p.Anomaly }) {
			anomalyOnly[strings.ToLower(c.Title)] = true
		}

		if err := addDatasetCard(&res, seen, c, expansions); err != nil {
			return Result{}, fmt.Errorf("%s: %w", c.ID, err)
		}
	}

	if len(res.Pool) == 0 {
		return Result{}, errors.New("the dataset holds no pool cards")
	}

	var rest []Set

	for _, s := range Sets {
		if _, ok := expansions[s.Slug]; !ok {
			rest = append(rest, s)
		}
	}

	if len(rest) > 0 {
		vexRes, err := buildSets(fsys, rest)
		if err != nil {
			return Result{}, err
		}

		mergeVex(&res, seen, anomalyOnly, vexRes)
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

// addDatasetCard adds one dataset card to the pool or the excluded list.
func addDatasetCard(
	res *Result,
	seen map[string]bool,
	c *card.Card,
	expansions map[string]int,
) error {
	if len(c.Printings) == 0 {
		return errors.New("the card has no printing")
	}

	p := firstPrinting(c)
	printings := make([]string, len(c.Printings))

	for i, pr := range c.Printings {
		printings[i] = pr.Set + "/" + pr.Number
	}

	if c.HasFlag("eviltwin") {
		res.Excluded = append(res.Excluded, Excluded{
			Name: c.Title, Set: p.Set, Number: p.Number, Type: c.Type, House: c.House,
			Reason: ReasonEvilTwin, Printings: printings,
		})

		return nil
	}

	entry, err := datasetEntry(c, p, expansions)
	if err != nil {
		return err
	}

	key := strings.ToLower(entry.Name)
	if seen[key] {
		return fmt.Errorf("the dataset has two cards named %q", entry.Name)
	}

	seen[key] = true
	res.PoolPrintings[entry.Name] = printings
	res.Pool = append(res.Pool, entry)

	return nil
}

// firstPrinting returns the card's first printing that is not an anomaly
// printing, or its first printing. The dataset lists the printings oldest
// first.
func firstPrinting(c *card.Card) *card.Printing {
	for i := range c.Printings {
		if !c.Printings[i].Anomaly {
			return &c.Printings[i]
		}
	}

	return &c.Printings[0]
}

// datasetEntry converts a dataset card to a pool entry, with the set, number,
// rarity and image of printing p.
func datasetEntry(c *card.Card, p *card.Printing, expansions map[string]int) (cards.Entry, error) {
	expansion, ok := expansions[p.Set]
	if !ok {
		return cards.Entry{}, fmt.Errorf("unknown set %q", p.Set)
	}

	reference := c.Type == "reference"

	cardType, ok := datasetTypes[c.Type]
	if reference {
		cardType, ok = referenceGroups[c.Group]
	}

	if !ok {
		return cards.Entry{}, fmt.Errorf("unknown card type %q (group %q)", c.Type, c.Group)
	}

	house := ""
	if c.House != "" && !reference {
		if house, ok = houses[c.House]; !ok {
			return cards.Entry{}, fmt.Errorf("unknown house %q", c.House)
		}
	}

	// The page orders only its own rarities. A token creature, an anomaly
	// and the tide are Special, as a reference card is.
	rarity := p.Rarity
	if reference || !rarities[rarity] {
		rarity = "Special"
	}

	out := cards.Card{
		Name:       c.Title,
		House:      house,
		CardType:   cardType,
		Rarity:     rarity,
		Amber:      c.Aember,
		Traits:     traits(c.Traits),
		CardText:   strings.ReplaceAll(derive.Display(c.Text), tideIcon, "Tide:"),
		CardNumber: p.Number,
		Expansion:  expansion,
		Image:      ImageBaseURL + p.Image,
	}

	// A creature always has power and armor, zero included.
	if cardType == "Creature" {
		power := "0"
		if c.Power != nil {
			power = c.Power.String()
		}

		armor := strconv.Itoa(c.Armor)
		out.Power = &power
		out.Armor = &armor
	}

	entry := cards.Entry{Card: out, Reference: reference}
	if c.Folded != "" && !strings.EqualFold(c.Folded, c.Title) {
		entry.Folded = c.Folded
	}

	return entry, nil
}

// mergeVex adds the vex cards that the dataset does not have, and replaces
// the dataset cards in anomalyOnly with their vex card.
func mergeVex(res *Result, seen, anomalyOnly map[string]bool, vexRes Result) {
	for _, e := range vexRes.Pool {
		key := strings.ToLower(e.Name)
		if anomalyOnly[key] {
			i := slices.IndexFunc(
				res.Pool,
				func(p cards.Entry) bool { return strings.EqualFold(p.Name, key) },
			)
			delete(res.PoolPrintings, res.Pool[i].Name)
			res.Pool = slices.Delete(res.Pool, i, i+1)
			delete(anomalyOnly, key)
		} else if seen[key] {
			continue
		}

		seen[key] = true
		res.PoolPrintings[e.Name] = vexRes.PoolPrintings[e.Name]
		res.Pool = append(res.Pool, e)
	}

	// A vex exclusion of a card that the pool has from the dataset, such as a
	// reprinted gigantic creature, is dropped. An evil twin keeps its
	// exclusion, because it has the name of a pool card.
	for _, e := range vexRes.Excluded {
		if e.Reason != ReasonEvilTwin && seen[strings.ToLower(e.Name)] {
			continue
		}

		res.Excluded = append(res.Excluded, e)
	}
}
