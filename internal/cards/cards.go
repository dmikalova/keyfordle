// Package cards holds the KeyFordle card pool.
//
// The pool is cards.json, embedded at build time. `mage cards` generates it
// from vex's set catalogs (see internal/catalog). A card's JSON field names are
// the ones the upstream KFStats CardSerializer used, because the lifted React
// page reads them.
package cards

import (
	"crypto/md5"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"
	"unicode"
)

//go:embed cards.json
var poolJSON []byte

// Card is one card as the API serves it.
type Card struct {
	Name       string   `json:"name"`
	House      string   `json:"house"`
	CardType   string   `json:"card_type"`
	Rarity     string   `json:"rarity"`
	Amber      int      `json:"amber"`
	Power      *string  `json:"power"`
	Armor      *string  `json:"armor"`
	Traits     []string `json:"traits"`
	CardText   string   `json:"card_text"`
	CardNumber string   `json:"card_number"`
	Expansion  int      `json:"expansion"`
	Image      string   `json:"image"`
}

// Entry is one card as cards.json stores it: the served card, the
// ASCII-folded name that search also matches, so "aember" finds "Æmber", and
// whether it is a reference card.
type Entry struct {
	Card

	Folded string `json:"folded,omitempty"`
	// Reference marks a card that sits beside a deck, not in it: an archon
	// power, a prophecy or the tide. Pick draws these on ReferencePercent of
	// days.
	Reference bool `json:"reference,omitempty"`
}

// ReferencePercent is the share of days, in percent, whose card is a
// reference card.
const ReferencePercent = 2

// Pool is the set of cards a player can guess and the day can pick.
type Pool struct {
	cards  []Card
	keys   []string
	byName map[string]int
	// deck and reference index cards, in name order, by the kind Pick draws.
	deck      []int
	reference []int
}

// Load parses the embedded pool.
func Load() (*Pool, error) {
	var entries []Entry
	if err := json.Unmarshal(poolJSON, &entries); err != nil {
		return nil, fmt.Errorf("parse cards.json: %w", err)
	}

	return New(entries)
}

// New validates entries and builds a pool. Entries must be non-empty, sorted by
// name and unique by name: the daily pick indexes into this order, so an
// unsorted pool would pick a different card for the same day.
func New(entries []Entry) (*Pool, error) {
	if len(entries) == 0 {
		return nil, errors.New("the card pool is empty")
	}

	p := &Pool{
		cards:  make([]Card, len(entries)),
		keys:   make([]string, len(entries)),
		byName: make(map[string]int, len(entries)),
	}

	for i, e := range entries {
		if strings.TrimSpace(e.Name) == "" {
			return nil, fmt.Errorf("card %d has no name", i)
		}

		if _, dup := p.byName[e.Name]; dup {
			return nil, fmt.Errorf("card %q is in the pool twice", e.Name)
		}

		if i > 0 && entries[i-1].Name > e.Name {
			return nil, fmt.Errorf("card %q is out of name order", e.Name)
		}

		p.cards[i] = e.Card
		p.keys[i] = strings.ToLower(e.Name + "\n" + e.Folded)
		p.byName[e.Name] = i

		if e.Reference {
			p.reference = append(p.reference, i)
		} else {
			p.deck = append(p.deck, i)
		}
	}

	if len(p.deck) == 0 {
		return nil, errors.New("the card pool holds only reference cards")
	}

	return p, nil
}

// Len is the number of cards in the pool.
func (p *Pool) Len() int { return len(p.cards) }

// ByName returns the card with exactly this name.
func (p *Pool) ByName(name string) (Card, bool) {
	i, ok := p.byName[name]
	if !ok {
		return Card{}, false
	}

	return p.cards[i], true
}

// Pick returns the card for a UTC day. It extends upstream KeyFordle's rule:
// the MD5 of the date as YYYYMMDD, read as an integer n. When n mod 100 is
// below ReferencePercent and the pool holds reference cards, n / 100 modulo
// their count indexes them in name order. Otherwise n modulo the deck card
// count indexes the deck cards in name order, which is upstream's rule for a
// pool without reference cards. MD5 is a spreader here, not a security
// boundary.
func (p *Pool) Pick(day time.Time) Card {
	sum := md5.Sum([]byte(day.UTC().Format("20060102")))
	n := new(big.Int).SetBytes(sum[:])

	quotient, percent := new(big.Int).DivMod(n, big.NewInt(100), new(big.Int))
	if len(p.reference) > 0 && percent.Int64() < ReferencePercent {
		return p.cards[p.reference[indexOf(quotient, len(p.reference))]]
	}

	return p.cards[p.deck[indexOf(n, len(p.deck))]]
}

// indexOf returns n modulo size.
func indexOf(n *big.Int, size int) int64 {
	return new(big.Int).Mod(n, big.NewInt(int64(size))).Int64()
}

// MinQueryLength is the shortest query Search answers, as upstream.
const MinQueryLength = 2

// The match tiers Search ranks by, best first.
const (
	matchContains = iota // the name contains the query as typed
	matchInOrder         // the query's letters appear in the name in order
	matchAnyOrder        // the query's letters all appear in the name
	matchNone
)

// Search returns up to limit cards whose name holds every letter of query,
// ignoring case, in any order. Matches rank by tier: the name contains the
// query, then the letters appear in order, then they appear in any order.
// Within a tier, cards keep name order. A letter typed twice must appear twice.
// Spaces and punctuation in the query only count for the contains tier. A
// query shorter than MinQueryLength runes matches nothing.
func (p *Pool) Search(query string, limit int) []Card {
	query = strings.ToLower(strings.TrimSpace(query))
	if len([]rune(query)) < MinQueryLength || limit <= 0 {
		return []Card{}
	}

	letters := lettersOf(query)
	tiers := make([][]Card, matchNone)

	for i, key := range p.keys {
		best := matchNone
		// A key holds the printed name and its ASCII fold; the better wins.
		for name := range strings.SplitSeq(key, "\n") {
			best = min(best, matchTier(name, query, letters))
		}

		if best != matchNone {
			tiers[best] = append(tiers[best], p.cards[i])
		}
	}

	out := make([]Card, 0, min(limit, 16))
	for _, tier := range tiers {
		out = append(out, tier[:min(len(tier), limit-len(out))]...)
	}

	return slices.Clip(out)
}

// matchTier reports how name matches a lowercased query and its letters.
func matchTier(name, query string, letters []rune) int {
	switch {
	case strings.Contains(name, query):
		return matchContains
	case len(letters) == 0:
		return matchNone
	case inOrder(name, letters):
		return matchInOrder
	case allPresent(name, letters):
		return matchAnyOrder
	default:
		return matchNone
	}
}

// lettersOf returns the letters and digits of s.
func lettersOf(s string) []rune {
	var out []rune

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out = append(out, r)
		}
	}

	return out
}

// inOrder reports whether letters appear in name in order, with gaps allowed.
func inOrder(name string, letters []rune) bool {
	i := 0

	for _, r := range name {
		if r == letters[i] {
			i++
			if i == len(letters) {
				return true
			}
		}
	}

	return false
}

// allPresent reports whether name holds every one of letters, counting
// repeats, in any order.
func allPresent(name string, letters []rune) bool {
	have := map[rune]int{}
	for _, r := range name {
		have[r]++
	}

	for _, r := range letters {
		if have[r] == 0 {
			return false
		}

		have[r]--
	}

	return true
}
