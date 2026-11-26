package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dmikalova/keyfordle/internal/cards"
	"github.com/dmikalova/keyfordle/internal/server"
	"github.com/dmikalova/keyfordle/internal/store"
)

// fakeStore keeps days in memory with the same rules as the SQL.
type fakeStore struct {
	mu   sync.Mutex
	days map[string]*store.Day
	fail error
}

func newFakeStore() *fakeStore { return &fakeStore{days: map[string]*store.Day{}} }

func (f *fakeStore) ClaimDay(_ context.Context, date, cardName string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.fail != nil {
		return "", f.fail
	}

	if d, ok := f.days[date]; ok {
		return d.CardName, nil
	}

	day, _ := time.Parse("2006-01-02", date)
	f.days[date] = &store.Day{Date: day, CardName: cardName}

	return cardName, nil
}

func (f *fakeStore) Day(_ context.Context, date string) (store.Day, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.fail != nil {
		return store.Day{}, f.fail
	}

	d, ok := f.days[date]
	if !ok {
		return store.Day{}, store.ErrNoDay
	}

	return *d, nil
}

func (f *fakeStore) RecordFinish(
	_ context.Context,
	date string,
	won bool,
	guesses int,
) (store.Day, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	d, ok := f.days[date]
	if !ok {
		return store.Day{}, store.ErrNoDay
	}

	d.GamesPlayed++
	d.TotalGuesses += guesses

	if won {
		d.Wins++
	} else {
		d.Losses++
	}

	return *d, nil
}

func (f *fakeStore) PopularDays(_ context.Context, limit int) ([]store.Day, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.fail != nil {
		return nil, f.fail
	}

	days := []store.Day{}

	for _, d := range f.days {
		if d.GamesPlayed > 0 {
			days = append(days, *d)
		}
	}

	slices.SortFunc(days, func(a, b store.Day) int {
		if a.GamesPlayed != b.GamesPlayed {
			return b.GamesPlayed - a.GamesPlayed
		}

		return b.Date.Compare(a.Date)
	})

	return days[:min(limit, len(days))], nil
}

var noon = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func newServer(t *testing.T, st *fakeStore) http.Handler {
	t.Helper()

	power := "3"
	zero := "0"

	pool, err := cards.New([]cards.Entry{
		{
			Name:     "Ancient Bear",
			House:    "Untamed",
			CardType: "Creature",
			Power:    &power,
			Armor:    &zero,
			Traits:   []string{},
		},
		{Name: "Bear Flute", House: "Untamed", CardType: "Artifact", Traits: []string{}},
		{
			Name:     "Ether Spider",
			House:    "Shadows",
			CardType: "Creature",
			Power:    &power,
			Armor:    &zero,
			Traits:   []string{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	site := http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "page") },
	)

	return server.New(server.Options{
		Pool:   pool,
		Store:  st,
		Site:   site,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() time.Time { return noon },
	}).Handler()
}

func do(t *testing.T, h http.Handler, r *http.Request) (int, map[string]any, http.Header) {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	var body map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s: decode %q: %v", r.Method, r.URL, rec.Body.String(), err)
		}
	}

	return rec.Code, body, rec.Header()
}

func post(body string) *http.Request {
	r := httptest.NewRequest(
		http.MethodPost,
		"/api/decks/public/record_keyfordle_completion/",
		strings.NewReader(body),
	)
	r.Header.Set("Content-Type", "application/json")

	return r
}

func TestDailyCardIsClaimedOnce(t *testing.T) {
	st := newFakeStore()
	h := newServer(t, st)

	code, body, hdr := do(
		t,
		h,
		httptest.NewRequest(http.MethodGet, "/api/decks/public/daily_card/", nil),
	)
	if code != http.StatusOK || body["date"] != "2026-11-25" {
		t.Fatalf("daily_card = %d %v", code, body)
	}

	if hdr.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", hdr.Get("Cache-Control"))
	}

	first := body["card"].(map[string]any)["name"]

	// A stored card wins over the pool's pick.
	st.days["2026-11-25"].CardName = "Bear Flute"

	_, body, _ = do(t, h, httptest.NewRequest(http.MethodGet, "/api/decks/public/daily_card/", nil))
	if got := body["card"].(map[string]any)["name"]; got != "Bear Flute" {
		t.Fatalf("daily_card after the claim = %v (first %v), want the stored card", got, first)
	}
}

func TestDailyCardFailures(t *testing.T) {
	st := newFakeStore()
	h := newServer(t, st)

	st.days["2026-11-25"] = &store.Day{CardName: "Removed From The Pool"}

	code, body, _ := do(
		t,
		h,
		httptest.NewRequest(http.MethodGet, "/api/decks/public/daily_card/", nil),
	)
	if code != http.StatusInternalServerError || body["error"] == nil {
		t.Fatalf("a stored card missing from the pool = %d %v", code, body)
	}

	st.fail = errors.New("database down")

	code, _, _ = do(t, h, httptest.NewRequest(http.MethodGet, "/api/decks/public/daily_card/", nil))
	if code != http.StatusInternalServerError {
		t.Fatalf("a store failure = %d", code)
	}
}

func TestCardSearch(t *testing.T) {
	h := newServer(t, newFakeStore())

	tests := []struct {
		query string
		want  int
	}{
		{"?search=bear&limit=10", 2},
		{"?search=bear&limit=1", 1},
		{"?search=bear&limit=nonsense", 2},
		{"?search=b", 0},
		{"", 0},
	}

	for _, tt := range tests {
		code, body, _ := do(
			t,
			h,
			httptest.NewRequest(http.MethodGet, "/api/decks/public/card_search/"+tt.query, nil),
		)
		results, ok := body["results"].([]any)

		if code != http.StatusOK || !ok || len(results) != tt.want {
			t.Errorf("card_search%s = %d %v, want %d results", tt.query, code, body, tt.want)
		}
	}
}

func TestCompletionCountsAndRanks(t *testing.T) {
	st := newFakeStore()
	h := newServer(t, st)

	code, body, _ := do(
		t,
		h,
		post(`{"card_name": "Ether Spider", "game_won": true, "num_guesses": 3}`),
	)
	if code != http.StatusOK || body["player_rank"] != 1.0 || body["wins"] != 1.0 {
		t.Fatalf("first completion = %d %v", code, body)
	}

	code, body, _ = do(
		t,
		h,
		post(`{"card_name": "Ether Spider", "game_won": false, "num_guesses": 6}`),
	)
	if code != http.StatusOK || body["player_rank"] != 2.0 || body["losses"] != 1.0 {
		t.Fatalf("second completion = %d %v", code, body)
	}

	if body["win_rate"] != 50.0 || body["average_guesses"] != 4.5 {
		t.Fatalf("stats = %v", body)
	}

	// The page's card_name never changes which card the day stores.
	if body["card_of_the_day"] == "Ether Spider" &&
		st.days["2026-11-25"].CardName != "Ether Spider" {
		t.Fatal("card_of_the_day came from the request")
	}
}

func TestCompletionValidation(t *testing.T) {
	h := newServer(t, newFakeStore())

	tests := map[string]string{
		"bad json":       `{`,
		"no card":        `{"game_won": true, "num_guesses": 1}`,
		"zero guesses":   `{"card_name": "X", "game_won": true, "num_guesses": 0}`,
		"too many":       `{"card_name": "X", "game_won": true, "num_guesses": 7}`,
		"string guesses": `{"card_name": "X", "game_won": true, "num_guesses": "3"}`,
	}

	for name, body := range tests {
		code, resp, _ := do(t, h, post(body))
		if code != http.StatusBadRequest || resp["error"] == nil {
			t.Errorf("%s: %d %v, want 400 with an error", name, code, resp)
		}
	}
}

func TestCompletionRejectsCrossOrigin(t *testing.T) {
	h := newServer(t, newFakeStore())

	r := post(`{"card_name": "X", "game_won": true, "num_guesses": 1}`)
	r.Host = "keyfordle.keyforge.cards"
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("Sec-Fetch-Site", "cross-site")

	if code, _, _ := do(t, h, r); code != http.StatusForbidden {
		t.Fatalf("a cross-origin POST = %d, want 403", code)
	}

	r = post(`{"card_name": "X", "game_won": true, "num_guesses": 1}`)
	r.Host = "keyfordle.keyforge.cards"
	r.Header.Set("Origin", "https://keyfordle.keyforge.cards")
	r.Header.Set("Sec-Fetch-Site", "same-origin")

	if code, _, _ := do(t, h, r); code != http.StatusOK {
		t.Fatalf("a same-origin POST = %d, want 200", code)
	}
}

func TestStats(t *testing.T) {
	st := newFakeStore()
	h := newServer(t, st)

	code, body, _ := do(
		t,
		h,
		httptest.NewRequest(http.MethodGet, "/api/decks/public/keyfordle_stats/", nil),
	)
	if code != http.StatusOK || body["card_of_the_day"] != nil || body["games_played"] != 0.0 ||
		body["date"] != "2026-11-25" {
		t.Fatalf("stats before any play = %d %v", code, body)
	}

	if _, ok := body["created_at"]; ok {
		t.Error("an empty day has created_at")
	}

	do(t, h, post(`{"card_name": "X", "game_won": true, "num_guesses": 2}`))

	_, body, _ = do(
		t,
		h,
		httptest.NewRequest(http.MethodGet, "/api/decks/public/keyfordle_stats/", nil),
	)
	if body["games_played"] != 1.0 || body["card_of_the_day"] == nil {
		t.Fatalf("stats after a play = %v", body)
	}

	st.fail = errors.New("database down")

	if code, _, _ := do(
		t,
		h,
		httptest.NewRequest(http.MethodGet, "/api/decks/public/keyfordle_stats/", nil),
	); code != http.StatusInternalServerError {
		t.Fatalf("stats on a store failure = %d", code)
	}
}

func TestRoutes(t *testing.T) {
	h := newServer(t, newFakeStore())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("/health = %d", rec.Code)
	}

	code, body, _ := do(t, h, httptest.NewRequest(http.MethodGet, "/api/nope/", nil))
	if code != http.StatusNotFound || body["error"] == nil {
		t.Errorf("an unknown API path = %d %v, want a JSON 404", code, body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/anything", nil))

	if rec.Code != http.StatusOK || rec.Body.String() != "page" {
		t.Errorf("/anything = %d %q, want the page", rec.Code, rec.Body.String())
	}
}

func TestPageRejectsWrites(t *testing.T) {
	h := newServer(t, newFakeStore())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("")))

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST / = %d Allow %q, want 405", rec.Code, rec.Header().Get("Allow"))
	}
}

// TestPastDays pins history mode: a past day has its own card and its own
// totals, and a day before FirstDay or after today is refused.
func TestPastDays(t *testing.T) {
	st := newFakeStore()
	h := newServer(t, st)

	get := func(path string) (int, map[string]any) {
		code, body, _ := do(t, h, httptest.NewRequest(http.MethodGet, path, nil))

		return code, body
	}

	code, body := get("/api/decks/public/daily_card/?date=2019-06-01")
	if code != http.StatusOK || body["date"] != "2019-06-01" || body["today"] != "2026-11-25" ||
		body["first_day"] != server.FirstDay {
		t.Fatalf("past daily_card = %d %v", code, body)
	}

	if _, ok := st.days["2019-06-01"]; !ok {
		t.Fatal("a past day's card was not stored")
	}

	code, body, _ = do(
		t,
		h,
		post(`{"card_name":"x","date":"2019-06-01","game_won":true,"num_guesses":2}`),
	)
	if code != http.StatusOK || body["date"] != "2019-06-01" || body["player_rank"] != 1.0 {
		t.Fatalf("past completion = %d %v", code, body)
	}

	if _, ok := st.days["2026-11-25"]; ok {
		t.Fatal("a past day's completion touched today")
	}

	code, body = get("/api/decks/public/keyfordle_stats/?date=2019-06-01")
	if code != http.StatusOK || body["games_played"] != 1.0 {
		t.Fatalf("past stats = %d %v", code, body)
	}

	for _, date := range []string{"2018-11-14", "2026-11-26", "2019-13-01", "yesterday"} {
		if code, _ := get(
			"/api/decks/public/daily_card/?date=" + date,
		); code != http.StatusBadRequest {
			t.Errorf("daily_card?date=%s = %d, want 400", date, code)
		}

		if code, _ := get(
			"/api/decks/public/keyfordle_stats/?date=" + date,
		); code != http.StatusBadRequest {
			t.Errorf("keyfordle_stats?date=%s = %d, want 400", date, code)
		}

		body := `{"card_name":"x","date":"` + date + `","game_won":true,"num_guesses":2}`
		if code, _, _ := do(t, h, post(body)); code != http.StatusBadRequest {
			t.Errorf("completion on %s = %d, want 400", date, code)
		}
	}

	if code, _ := get(
		"/api/decks/public/daily_card/?date=" + server.FirstDay,
	); code != http.StatusOK {
		t.Errorf("daily_card on FirstDay = %d", code)
	}
}

func TestPopularDays(t *testing.T) {
	st := newFakeStore()
	h := newServer(t, st)

	plays := map[string]int{"2019-01-01": 1, "2020-01-01": 3, "2021-01-01": 2}
	for date, n := range plays {
		for range n {
			body := `{"card_name":"x","date":"` + date + `","game_won":false,"num_guesses":6}`
			if code, _, _ := do(t, h, post(body)); code != http.StatusOK {
				t.Fatalf("completion on %s = %d", date, code)
			}
		}
	}

	// A day claimed but never finished is not popular.
	do(
		t,
		h,
		httptest.NewRequest(http.MethodGet, "/api/decks/public/daily_card/?date=2022-01-01", nil),
	)

	code, body, _ := do(
		t,
		h,
		httptest.NewRequest(http.MethodGet, "/api/decks/public/keyfordle_days/", nil),
	)
	if code != http.StatusOK {
		t.Fatalf("keyfordle_days = %d %v", code, body)
	}

	days, ok := body["days"].([]any)
	if !ok {
		t.Fatalf("keyfordle_days body has no days list: %v", body)
	}

	var got []string

	for _, d := range days {
		day, ok := d.(map[string]any)
		if !ok {
			t.Fatalf("popular day is not an object: %v", d)
		}

		date, _ := day["date"].(string)
		got = append(got, date)

		if day["card_of_the_day"] != nil {
			t.Errorf("popular day %v names its card", day["date"])
		}
	}

	want := []string{"2020-01-01", "2021-01-01", "2019-01-01"}
	if !slices.Equal(got, want) {
		t.Fatalf("popular days = %v, want %v", got, want)
	}

	st.fail = errors.New("down")

	if code, _, _ := do(
		t,
		h,
		httptest.NewRequest(http.MethodGet, "/api/decks/public/keyfordle_days/", nil),
	); code != http.StatusInternalServerError {
		t.Fatalf("keyfordle_days with the store down = %d", code)
	}
}
