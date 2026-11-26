package cards_test

import (
	"crypto/md5"
	"math/big"
	"testing"
	"time"

	"github.com/dmikalova/keyfordle/internal/cards"
)

func entry(name, folded string) cards.Entry {
	return cards.Entry{Name: name, Folded: folded}
}

func TestLoadEmbeddedPool(t *testing.T) {
	pool, err := cards.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if pool.Len() < 1000 {
		t.Fatalf("the embedded pool has %d cards; run `mage cards`", pool.Len())
	}
}

func TestNewRejectsABadPool(t *testing.T) {
	tests := map[string][]cards.Entry{
		"empty":        nil,
		"no name":      {entry("", "")},
		"duplicate":    {entry("A", ""), entry("A", "")},
		"out of order": {entry("B", ""), entry("A", "")},
	}

	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := cards.New(entries); err == nil {
				t.Fatal("New accepted the pool")
			}
		})
	}
}

func TestPickFollowsTheUpstreamRule(t *testing.T) {
	names := []string{"A", "B", "C", "D", "E", "F", "G"}

	entries := make([]cards.Entry, 0, len(names))
	for _, n := range names {
		entries = append(entries, entry(n, ""))
	}

	pool, err := cards.New(entries)
	if err != nil {
		t.Fatal(err)
	}

	// 23:00 at UTC-8 on the 25th is 07:00 UTC on the 26th: the UTC date counts.
	day := time.Date(2026, 11, 25, 23, 0, 0, 0, time.FixedZone("UTC-8", -8*3600))

	// Upstream: int(md5("YYYYMMDD").hexdigest(), 16) % len(cards ordered by name).
	sum := md5.Sum([]byte("20261126"))
	n := new(big.Int).SetBytes(sum[:])
	want := names[n.Mod(n, big.NewInt(int64(len(names)))).Int64()]

	if got := pool.Pick(day).Name; got != want {
		t.Fatalf("Pick = %q, want %q", got, want)
	}
}

// TestPickDrawsReferenceCards pins the reference card rule: when the date's MD5
// is below ReferencePercent modulo 100, the day draws a reference card, and
// otherwise a deck card by upstream's rule over the deck cards alone.
func TestPickDrawsReferenceCards(t *testing.T) {
	deck := []string{"A", "C", "D", "E", "G"}
	reference := []string{"B", "F"}
	isReference := map[string]bool{"B": true, "F": true}

	entries := make([]cards.Entry, 0, 7)
	for _, n := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		e := entry(n, "")
		e.Reference = isReference[n]
		entries = append(entries, e)
	}

	pool, err := cards.New(entries)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(2018, 11, 15, 0, 0, 0, 0, time.UTC)
	days, referenceDays := 3650, 0

	for d := range days {
		day := start.AddDate(0, 0, d)
		sum := md5.Sum([]byte(day.Format("20060102")))
		n := new(big.Int).SetBytes(sum[:])
		quotient, percent := new(big.Int).DivMod(n, big.NewInt(100), new(big.Int))

		var want string
		if percent.Int64() < cards.ReferencePercent {
			referenceDays++
			want = reference[new(big.Int).Mod(quotient, big.NewInt(2)).Int64()]
		} else {
			want = deck[new(big.Int).Mod(n, big.NewInt(5)).Int64()]
		}

		if got := pool.Pick(day).Name; got != want {
			t.Fatalf("%s: Pick = %q, want %q", day.Format(time.DateOnly), got, want)
		}
	}

	// About 2% of ten years is 73 days.
	if referenceDays < 40 || referenceDays > 110 {
		t.Fatalf("%d of %d days drew a reference card, want about 2%%", referenceDays, days)
	}
}

func TestNewRejectsAReferenceOnlyPool(t *testing.T) {
	e := entry("A", "")
	e.Reference = true

	if _, err := cards.New([]cards.Entry{e}); err == nil {
		t.Fatal("New accepted a pool with no deck cards")
	}
}

func TestPickIsStableAndSpread(t *testing.T) {
	entries := make([]cards.Entry, 0, 26)
	for c := 'A'; c <= 'Z'; c++ {
		entries = append(entries, entry(string(c), ""))
	}

	pool, err := cards.New(entries)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	seen := map[string]bool{}

	for d := range 60 {
		day := start.AddDate(0, 0, d)
		// Noon and 23:59 on the same UTC day pick the same card.
		if pool.Pick(day).Name != pool.Pick(day.Add(11*time.Hour+59*time.Minute)).Name {
			t.Fatalf("day %d picked two cards", d)
		}

		seen[pool.Pick(day).Name] = true
	}

	if len(seen) < 10 {
		t.Fatalf("60 days picked only %d distinct cards", len(seen))
	}
}

func TestByName(t *testing.T) {
	pool, err := cards.New([]cards.Entry{entry("Æmber Imp", "Aember Imp")})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := pool.ByName("Æmber Imp"); !ok {
		t.Fatal("ByName missed the printed name")
	}

	if _, ok := pool.ByName("Aember Imp"); ok {
		t.Fatal("ByName matched the folded name; names must match exactly")
	}
}

// TestSearchAnyOrder pins the fuzzy ranking: contains, then letters in order,
// then letters in any order, each tier in name order.
func TestSearchAnyOrder(t *testing.T) {
	pool, err := cards.New([]cards.Entry{
		entry("Dodger", ""),
		entry("Hand of Dis", ""),
		entry("Overshadow", ""),
		entry("Shadow Self", ""),
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query string
		want  []string
	}{
		// Overshadow and Shadow Self contain it; Hand of Dis has h-a-d-o in
		// order (H, a, d, o); nothing else holds all four letters.
		{"hado", []string{"Overshadow", "Shadow Self", "Hand of Dis"}},
		// No name contains "odah" or has it in order; three hold the letters.
		{"odah", []string{"Hand of Dis", "Overshadow", "Shadow Self"}},
		// A repeated letter must appear twice: only Shadow Self has two s's.
		{"ss", []string{"Shadow Self"}},
		// Spaces in the query only matter for the contains tier.
		{"self sha", []string{"Shadow Self"}},
		{"xyz", nil},
	}

	for _, tt := range tests {
		got := pool.Search(tt.query, 10)

		names := make([]string, len(got))
		for i, c := range got {
			names[i] = c.Name
		}

		if len(names) != len(tt.want) {
			t.Fatalf("Search(%q) = %v, want %v", tt.query, names, tt.want)
		}

		for i := range names {
			if names[i] != tt.want[i] {
				t.Fatalf("Search(%q) = %v, want %v", tt.query, names, tt.want)
			}
		}
	}

	if got := pool.Search("odah", 2); len(got) != 2 {
		t.Fatalf("Search limit 2 returned %d cards", len(got))
	}
}

func TestSearch(t *testing.T) {
	pool, err := cards.New([]cards.Entry{
		entry("Ancient Bear", ""),
		entry("Bear Flute", ""),
		entry("Æmber Imp", "Aember Imp"),
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query string
		limit int
		want  []string
	}{
		// Æmber Imp folds to "aember imp", which holds b, e, a and r.
		{"bear", 10, []string{"Ancient Bear", "Bear Flute", "Æmber Imp"}},
		{"BEAR", 1, []string{"Ancient Bear"}},
		{"aember", 10, []string{"Æmber Imp"}},
		{"æmber", 10, []string{"Æmber Imp"}},
		{"  im  ", 10, []string{"Æmber Imp"}},
		{"b", 10, nil},
		{"Æ", 10, nil},
		{"bear", 0, nil},
		{"zzz", 10, nil},
	}

	for _, tt := range tests {
		got := pool.Search(tt.query, tt.limit)
		if got == nil {
			t.Fatalf("Search(%q) returned nil; the API needs []", tt.query)
		}

		if len(got) != len(tt.want) {
			t.Fatalf("Search(%q, %d) = %d cards, want %v", tt.query, tt.limit, len(got), tt.want)
		}

		for i := range got {
			if got[i].Name != tt.want[i] {
				t.Fatalf("Search(%q)[%d] = %q, want %q", tt.query, i, got[i].Name, tt.want[i])
			}
		}
	}
}
