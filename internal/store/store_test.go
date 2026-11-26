package store_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

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
			name, err := s.ClaimDay(ctx, date, string(rune('A'+i)))
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

func TestRecordFinishCountsEveryGame(t *testing.T) {
	s := open(t)
	ctx := context.Background()

	const date = "1999-01-02"

	if _, err := s.RecordFinish(ctx, "1999-01-03", true, 1); !errors.Is(err, store.ErrNoDay) {
		t.Fatalf("RecordFinish on an unclaimed day = %v, want ErrNoDay", err)
	}

	if _, err := s.ClaimDay(ctx, date, "Ancient Bear"); err != nil {
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
