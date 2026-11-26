package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dmikalova/keyfordle/internal/store"
)

// The store tests need a Postgres with db/schema.hcl applied. They run when
// KEYFORDLE_TEST_DATABASE_URL names one; README.md says how to start one.
func open(t *testing.T) *store.Store {
	t.Helper()

	url := os.Getenv("KEYFORDLE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KEYFORDLE_TEST_DATABASE_URL is not set")
	}

	s, err := store.Open(context.Background(), url)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(s.Close)

	return s
}

func TestClaimDayKeepsTheFirstCard(t *testing.T) {
	s := open(t)
	ctx := context.Background()

	const date = "1999-01-01"

	var wg sync.WaitGroup

	got := make([]string, 8)

	for i := range got {
		wg.Go(func() {
			name, err := s.ClaimDay(ctx, date, []string{string(rune('A' + i))})
			if err != nil {
				t.Errorf("ClaimDay: %v", err)
			}

			got[i] = name
		})
	}

	wg.Wait()

	for _, name := range got {
		if name != got[0] {
			t.Fatalf("concurrent claims read different cards: %v", got)
		}
	}
}

// TestClaimDaySkipsTakenCards claims three new days from the same two
// candidates. The second day skips the card the first day holds. The third
// finds both taken and takes the earliest of the least used.
func TestClaimDaySkipsTakenCards(t *testing.T) {
	s := open(t)
	ctx := context.Background()

	// The test database is reused, so the names are new each run and the days
	// are the first three unclaimed ones from 1900.
	run := time.Now().UnixNano()
	x, y := fmt.Sprintf("X %d", run), fmt.Sprintf("Y %d", run)
	base := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)

	for {
		_, err := s.Day(ctx, base.AddDate(0, 0, 2).Format(time.DateOnly))
		if errors.Is(err, store.ErrNoDay) {
			break
		}

		if err != nil {
			t.Fatal(err)
		}

		base = base.AddDate(0, 0, 3)
	}

	for i, want := range []string{x, y, x} {
		date := base.AddDate(0, 0, i).Format(time.DateOnly)

		got, err := s.ClaimDay(ctx, date, []string{x, y})
		if err != nil {
			t.Fatal(err)
		}

		if got != want {
			t.Fatalf("day %d took %q, want %q", i, got, want)
		}
	}

	if _, err := s.ClaimDay(ctx, base.Format(time.DateOnly), nil); err != nil {
		t.Fatalf("ClaimDay on a claimed day with no candidates: %v", err)
	}
}

func TestRecordFinishCountsEveryGame(t *testing.T) {
	s := open(t)
	ctx := context.Background()

	const date = "1999-01-02"

	if _, err := s.RecordFinish(ctx, "1999-01-03", true, 1); !errors.Is(err, store.ErrNoDay) {
		t.Fatalf("RecordFinish on an unclaimed day = %v, want ErrNoDay", err)
	}

	if _, err := s.ClaimDay(ctx, date, []string{"Ancient Bear"}); err != nil {
		t.Fatal(err)
	}

	before, err := s.Day(ctx, date)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	for i := range 10 {
		wg.Go(func() {
			if _, err := s.RecordFinish(ctx, date, i%2 == 0, 3); err != nil {
				t.Errorf("RecordFinish: %v", err)
			}
		})
	}

	wg.Wait()

	after, err := s.Day(ctx, date)
	if err != nil {
		t.Fatal(err)
	}

	if after.GamesPlayed-before.GamesPlayed != 10 ||
		after.Wins-before.Wins != 5 ||
		after.TotalGuesses-before.TotalGuesses != 30 {
		t.Fatalf("before %+v after %+v: lost a count", before, after)
	}

	if _, err := s.Day(ctx, "1999-01-04"); !errors.Is(err, store.ErrNoDay) {
		t.Fatalf("Day on an unclaimed day = %v, want ErrNoDay", err)
	}
}

func TestPopularDaysOrdersByPlays(t *testing.T) {
	s := open(t)
	ctx := context.Background()

	// The test database is reused, so each run adds plays: 3 on the first day
	// and 5 on the second keep the second ahead whatever ran before.
	for date, plays := range map[string]int{"1998-02-01": 3, "1998-02-02": 5} {
		if _, err := s.ClaimDay(ctx, date, []string{"Card"}); err != nil {
			t.Fatal(err)
		}

		for range plays {
			if _, err := s.RecordFinish(ctx, date, true, 2); err != nil {
				t.Fatal(err)
			}
		}
	}

	days, err := s.PopularDays(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}

	rank := map[string]int{}

	for i, d := range days {
		if d.GamesPlayed <= 0 {
			t.Fatalf("PopularDays returned %s with no games", d.Date.Format("2006-01-02"))
		}

		if i > 0 && d.GamesPlayed > days[i-1].GamesPlayed {
			t.Fatalf("PopularDays is not most played first: %+v", days)
		}

		rank[d.Date.Format("2006-01-02")] = i
	}

	first, ok1 := rank["1998-02-01"]
	second, ok2 := rank["1998-02-02"]

	if !ok1 || !ok2 || second > first {
		t.Fatalf("ranks: 1998-02-01=%d (%v), 1998-02-02=%d (%v)", first, ok1, second, ok2)
	}
}
