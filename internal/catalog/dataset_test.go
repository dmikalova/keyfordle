package catalog_test

import (
	"testing"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/sets"

	"github.com/dmikalova/keyfordle/internal/cards"
	"github.com/dmikalova/keyfordle/internal/catalog"
)

// dataset has the first two sets, with a card of every kind that the
// dataset gives: a deck card, an evil twin, a token creature, a gigantic
// creature, the tide, and an anomaly that vex has from a later set.
func dataset() *card.Dataset {
	five := card.Power{N: 5}
	six := card.Power{N: 6}

	return &card.Dataset{
		Sets: []sets.Set{
			{Slug: "callofthearchons", Expansion: 341},
			{Slug: "ageofascension", Expansion: 435},
		},
		Cards: []card.Card{
			{
				ID: "troll-brobnar", Title: "Troll", House: "brobnar", Type: "creature",
				Text: "Reap: Gain 1A.", Power: &five, Armor: 1,
				Printings: []card.Printing{
					{
						Set:    "callofthearchons",
						Number: "048",
						Rarity: "Common",
						Image:  "callofthearchons/048.webp?v=1",
					},
				},
			},
			{
				ID:     "troll-eviltwin",
				Title:  "Troll",
				House:  "brobnar",
				Type:   "creature",
				Text:   "Fight: Steal 1A.",
				Power:  &six,
				Folded: "troll",
				Flags:  []string{"eviltwin"},
				Printings: []card.Printing{
					{
						Set:    "ageofascension",
						Number: "300",
						Rarity: "Evil Twin",
						Image:  "ageofascension/300.webp?v=2",
					},
				},
			},
			{
				ID:    "grumpus-dis",
				Title: "Grumpus",
				House: "dis",
				Type:  "creature",
				Power: &five,
				Flags: []string{"tokencreature"},
				Printings: []card.Printing{
					{Set: "ageofascension", Number: "T01", Rarity: "Token Creature"},
				},
			},
			{
				ID:    "thetide",
				Title: "The Tide",
				Type:  "reference",
				Group: "thetide",
				Flags: []string{"reference"},
				Printings: []card.Printing{
					{Set: "ageofascension", Number: "000", Rarity: "The Tide"},
				},
			},
			{
				ID:    "orb",
				Title: "Orb",
				Type:  "artifact",
				Printings: []card.Printing{
					{Set: "ageofascension", Number: "A1", Rarity: "Anomaly", Anomaly: true},
				},
			},
		},
	}
}

// later is vex's catalog of a set that the dataset does not have.
var later = map[string]string{
	"callofthearchons": `[{"number": "999", "name": "Stale", "house": "dis", "type": "action", "rarity": "Common"}]`,
	"grimreminders": `[
		{"number": "001", "name": "Orb", "house": "geistoid", "type": "artifact", "rarity": "Rare"},
		{"number": "002", "name": "Troll", "house": "brobnar", "type": "creature", "rarity": "Common", "power": 8},
		{"number": "003", "name": "Ghost", "house": "geistoid", "type": "creature", "rarity": "Common", "power": 2}
	]`,
}

func byName(t *testing.T, pool []cards.Entry) map[string]cards.Entry {
	t.Helper()

	out := map[string]cards.Entry{}
	for _, e := range pool {
		out[e.Name] = e
	}

	return out
}

func TestBuildWithDataset(t *testing.T) {
	res, err := catalog.BuildWithDataset(dataset(), vex(later))
	if err != nil {
		t.Fatal(err)
	}

	pool := byName(t, res.Pool)

	troll := pool["Troll"]
	if troll.Expansion != 341 || *troll.Power != "5" || *troll.Armor != "1" ||
		troll.CardText != "Reap: Gain 1 Æmber." ||
		troll.Image != catalog.ImageBaseURL+"callofthearchons/048.webp?v=1" {
		t.Errorf("Troll = %+v, want the dataset card", troll)
	}

	if g := pool["Grumpus"]; g.CardType != "Creature" || g.Rarity != "Special" || g.House != "Dis" {
		t.Errorf("Grumpus = %+v, want a Special Dis creature", g)
	}

	if tide := pool["The Tide"]; !tide.Reference || tide.CardType != "The Tide" ||
		tide.House != "" {
		t.Errorf("The Tide = %+v, want a reference card", tide)
	}

	if orb := pool["Orb"]; orb.Expansion != 700 || orb.House != "Geistoid" {
		t.Errorf("Orb = %+v, want vex's Grim Reminders card", orb)
	}

	if _, ok := pool["Ghost"]; !ok {
		t.Error("the pool has no Ghost from a set that the dataset does not have")
	}

	if _, ok := pool["Stale"]; ok {
		t.Error("the pool read vex's catalog of a set that the dataset has")
	}

	twin := pool["Troll (Evil Twin)"]
	if twin.House != "Brobnar" || *twin.Power != "6" || twin.CardText != "Fight: Steal 1 Æmber." ||
		twin.Rarity != "Special" || twin.Folded != "troll (Evil Twin)" ||
		twin.Image != catalog.ImageBaseURL+"ageofascension/300.webp?v=2" {
		t.Errorf("Troll (Evil Twin) = %+v, want the evil twin card", twin)
	}

	if len(res.Pool) != 6 {
		t.Errorf("the pool has %d cards, want 6", len(res.Pool))
	}

	for _, e := range res.Excluded {
		if e.Reason == catalog.ReasonEvilTwin {
			t.Errorf("the excluded list has the evil twin %q", e.Name)
		}
	}
}
