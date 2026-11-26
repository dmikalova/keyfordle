// Package cards holds the Keyfordle card pool.
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

// Entry is one card as cards.json stores it: the served card plus the
// ASCII-folded name that search also matches, so "aember" finds "Æmber".
type Entry struct {
	Card

	Folded string `json:"folded,omitempty"`
}

// Pool is the set of cards a player can guess and the day can pick.
type Pool struct {
	cards  []Card
	keys   []string
	byName map[string]int
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

// Pick returns the card for a UTC day. It is upstream Keyfordle's rule: the MD5
// of the date as YYYYMMDD, read as an integer, modulo the pool size, indexes the
// pool in name order. MD5 is a spreader here, not a security boundary.
func (p *Pool) Pick(day time.Time) Card {
	sum := md5.Sum([]byte(day.UTC().Format("20060102")))
	n := new(big.Int).SetBytes(sum[:])
	i := n.Mod(n, big.NewInt(int64(len(p.cards)))).Int64()

	return p.cards[i]
}

// MinQueryLength is the shortest query Search answers, as upstream.
const MinQueryLength = 2

// Search returns up to limit cards whose name contains query, ignoring case, in
// name order. A query shorter than MinQueryLength runes matches nothing.
func (p *Pool) Search(query string, limit int) []Card {
	query = strings.ToLower(strings.TrimSpace(query))
	if len([]rune(query)) < MinQueryLength || limit <= 0 {
		return []Card{}
	}

	out := make([]Card, 0, min(limit, 16))

	for i, key := range p.keys {
		if strings.Contains(key, query) {
			out = append(out, p.cards[i])
			if len(out) == limit {
				break
			}
		}
	}

	return slices.Clip(out)
}
