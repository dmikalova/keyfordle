// Package server exposes Keyfordle over HTTP.
//
// The API keeps the upstream KFStats paths and JSON shapes, because the lifted
// React page calls them unchanged. Everything that is not the API or /health is
// the page.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dmikalova/keyfordle/internal/cards"
	"github.com/dmikalova/keyfordle/internal/store"
)

// MaxGuesses is the page's MAX_GUESSES. A finished game reports between one and
// this many guesses.
const MaxGuesses = 6

// Search limits. Upstream's default was 20; the page asks for the cap, so its
// dropdown can fill the window. The cap keeps one request from returning the
// whole pool.
const (
	defaultSearchLimit = 20
	maxSearchLimit     = 50
)

// maxBodyBytes bounds a finished-game report. The real body is under 100 bytes.
const maxBodyBytes = 4 << 10

// dateLayout is how a day is written in the API and in SQL.
const dateLayout = "2006-01-02"

// Store is the state the API reads and writes. *store.Store implements it.
type Store interface {
	ClaimDay(ctx context.Context, date, cardName string) (string, error)
	Day(ctx context.Context, date string) (store.Day, error)
	RecordFinish(ctx context.Context, date string, won bool, guesses int) (store.Day, error)
}

// Server wires the pool and the store to the routes.
type Server struct {
	pool  *cards.Pool
	store Store
	site  http.Handler
	log   *slog.Logger
	now   func() time.Time
}

// Options configures a Server.
type Options struct {
	Pool   *cards.Pool
	Store  Store
	Site   http.Handler
	Logger *slog.Logger
	// Now is the clock. It defaults to time.Now; tests set it.
	Now func() time.Time
}

// New returns a Server.
func New(o Options) *Server {
	now := o.Now
	if now == nil {
		now = time.Now
	}

	return &Server{pool: o.Pool, store: o.Store, site: o.Site, log: o.Logger, now: now}
}

// Handler returns the routed handler. Cross-origin protection rejects a POST
// whose Origin or Sec-Fetch-Site says it came from another site. It replaces
// upstream's Django CSRF token: the page and the API share one origin.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/decks/public/daily_card/{$}", s.handleDailyCard)
	mux.HandleFunc("GET /api/decks/public/card_search/{$}", s.handleCardSearch)
	mux.HandleFunc("POST /api/decks/public/record_keyfordle_completion/{$}", s.handleCompletion)
	mux.HandleFunc("GET /api/decks/public/keyfordle_stats/{$}", s.handleStats)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "Not found")
	})
	mux.HandleFunc("/", s.handlePage)

	return http.NewCrossOriginProtection().Handler(mux)
}

// handlePage serves the page for GET and HEAD. It is registered without a
// method so that /api/ stays the more specific pattern for every method.
func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)

		return
	}

	s.site.ServeHTTP(w, r)
}

// handleHealth is liveness only. It does not touch the database, so a database
// outage does not make Cloud Run replace a healthy instance.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

// today is the current UTC date. Upstream ran in UTC too.
func (s *Server) today() string { return s.now().UTC().Format(dateLayout) }

// claimToday returns today's card, storing the pool's pick if the day is new.
func (s *Server) claimToday(ctx context.Context) (cards.Card, error) {
	now := s.now()
	date := now.UTC().Format(dateLayout)

	name, err := s.store.ClaimDay(ctx, date, s.pool.Pick(now).Name)
	if err != nil {
		return cards.Card{}, err
	}

	card, ok := s.pool.ByName(name)
	if !ok {
		return cards.Card{}, errors.New(
			"the stored card " + strconv.Quote(name) + " is not in the pool",
		)
	}

	return card, nil
}

func (s *Server) handleDailyCard(w http.ResponseWriter, r *http.Request) {
	card, err := s.claimToday(r.Context())
	if err != nil {
		s.log.Error("daily card", "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to get daily card")

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"card": card, "date": s.today()})
}

func (s *Server) handleCardSearch(w http.ResponseWriter, r *http.Request) {
	limit := defaultSearchLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = min(n, maxSearchLimit)
		}
	}

	results := s.pool.Search(r.URL.Query().Get("search"), limit)
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// completion is the page's finished-game report. card_name is required, as
// upstream, but the stored card is the day's card whatever the page sends.
type completion struct {
	CardName   string `json:"card_name"`
	GameWon    bool   `json:"game_won"`
	NumGuesses int    `json:"num_guesses"`
}

func (s *Server) handleCompletion(w http.ResponseWriter, r *http.Request) {
	var c completion

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")

		return
	}

	switch {
	case strings.TrimSpace(c.CardName) == "":
		writeError(w, http.StatusBadRequest, "card_name is required")

		return
	case c.NumGuesses <= 0:
		writeError(w, http.StatusBadRequest, "num_guesses must be greater than 0")

		return
	case c.NumGuesses > MaxGuesses:
		writeError(
			w,
			http.StatusBadRequest,
			"num_guesses must be at most "+strconv.Itoa(MaxGuesses),
		)

		return
	}

	ctx := r.Context()

	if _, err := s.claimToday(ctx); err != nil {
		s.log.Error("record completion: claim the day", "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to record completion")

		return
	}

	day, err := s.store.RecordFinish(ctx, s.today(), c.GameWon, c.NumGuesses)
	if err != nil {
		s.log.Error("record completion", "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to record completion")

		return
	}

	out := statsOf(day)
	out.PlayerRank = day.GamesPlayed
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	date := s.today()

	day, err := s.store.Day(r.Context(), date)
	if errors.Is(err, store.ErrNoDay) {
		writeJSON(w, http.StatusOK, stats{Date: date})

		return
	}

	if err != nil {
		s.log.Error("stats", "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to get stats")

		return
	}

	writeJSON(w, http.StatusOK, statsOf(day))
}

// stats is upstream's KeyfordleStatsSerializer, plus player_rank on a recorded
// completion. A day with no row has a null card and zeros, and no timestamps.
type stats struct {
	Date           string     `json:"date"`
	CardOfTheDay   *string    `json:"card_of_the_day"`
	GamesPlayed    int        `json:"games_played"`
	Wins           int        `json:"wins"`
	Losses         int        `json:"losses"`
	TotalGuesses   int        `json:"total_guesses"`
	WinRate        float64    `json:"win_rate"`
	AverageGuesses float64    `json:"average_guesses"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
	PlayerRank     int        `json:"player_rank,omitempty"`
}

func statsOf(d store.Day) stats {
	// Upstream wrote its timestamps in UTC.
	d.CreatedAt = d.CreatedAt.UTC()
	d.UpdatedAt = d.UpdatedAt.UTC()

	out := stats{
		Date:         d.Date.Format(dateLayout),
		CardOfTheDay: &d.CardName,
		GamesPlayed:  d.GamesPlayed,
		Wins:         d.Wins,
		Losses:       d.Losses,
		TotalGuesses: d.TotalGuesses,
		CreatedAt:    &d.CreatedAt,
		UpdatedAt:    &d.UpdatedAt,
	}

	if d.GamesPlayed > 0 {
		out.WinRate = float64(d.Wins) / float64(d.GamesPlayed) * 100
		out.AverageGuesses = float64(d.TotalGuesses) / float64(d.GamesPlayed)
	}

	return out
}

// writeJSON writes v. API answers are never cached: they change with every
// finished game and every new day.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
