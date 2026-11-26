// Package store holds keyfordle's state in Postgres: one row per day.
//
// Every write is a single statement. The day's card is claimed with an insert
// that does nothing on conflict, and a finished game is counted with one
// UPDATE ... RETURNING, so two Cloud Run instances cannot lose a count or pick
// two cards for one day.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoDay means the day has no row yet.
var ErrNoDay = errors.New("no row for that day")

// Store is a connection pool plus the queries keyfordle needs.
type Store struct {
	pool *pgxpool.Pool
}

// Day is one day's card and its finished-game totals.
type Day struct {
	Date         time.Time
	CardName     string
	GamesPlayed  int
	Wins         int
	Losses       int
	TotalGuesses int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Open connects to Postgres through Supabase's transaction pooler.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse the database URL: %w", err)
	}

	// Supabase's transaction pooler hands a different backend to each
	// statement, so a prepared statement cached on one is absent on the next.
	// The simple protocol sends the whole statement every time.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.StatementCacheCapacity = 0
	cfg.ConnConfig.DescriptionCacheCapacity = 0

	// Cloud Run scales to zero and the pooler charges for idle connections.
	cfg.MaxConns = 4
	cfg.MaxConnIdleTime = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to the database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping the database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// dayColumns is the column list every query that returns a Day selects.
const dayColumns = `date, card_name, games_played, wins, losses, total_guesses, created_at, updated_at`

// ClaimDay returns the card stored for date. When the day has no row yet, it
// stores one of candidates first: the first that no other day holds, or, when
// every candidate is taken, the one that the fewest days hold, earliest first.
// A concurrent claim for the same day is harmless: the insert that loses does
// nothing, and both callers read the winner's card. Two new days claimed at the
// same moment can both take one card; nothing else repeats a card while an
// unused candidate is left.
func (s *Store) ClaimDay(ctx context.Context, date string, candidates []string) (string, error) {
	// Almost every call is for a day that already has its card. Reading it
	// first keeps the candidate list off the wire for those.
	stored, err := s.dayCard(ctx, date)
	if err == nil {
		return stored, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("read the card for %s: %w", date, err)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("claim the card for %s: no candidates", date)
	}

	const insert = `
		INSERT INTO keyfordle.days (date, card_name)
		SELECT $1::date, c.name
		FROM unnest($2::text[]) WITH ORDINALITY AS c(name, rank)
		LEFT JOIN (
			SELECT card_name, count(*) AS uses
			FROM keyfordle.days
			GROUP BY card_name
		) AS u ON u.card_name = c.name
		ORDER BY coalesce(u.uses, 0), c.rank
		LIMIT 1
		ON CONFLICT (date) DO NOTHING`

	if _, err := s.pool.Exec(ctx, insert, date, candidates); err != nil {
		return "", fmt.Errorf("claim the card for %s: %w", date, err)
	}

	// A separate statement, so its snapshot sees a row another instance
	// committed while the insert above waited on it.
	stored, err = s.dayCard(ctx, date)
	if err != nil {
		return "", fmt.Errorf("read the card for %s: %w", date, err)
	}

	return stored, nil
}

// dayCard reads the card stored for date.
func (s *Store) dayCard(ctx context.Context, date string) (string, error) {
	var stored string

	const query = `SELECT card_name FROM keyfordle.days WHERE date = $1::date`
	err := s.pool.QueryRow(ctx, query, date).Scan(&stored)

	return stored, err
}

// Day returns the row for date, or ErrNoDay.
func (s *Store) Day(ctx context.Context, date string) (Day, error) {
	query := `SELECT ` + dayColumns + ` FROM keyfordle.days WHERE date = $1::date`

	d, err := scanDay(s.pool.QueryRow(ctx, query, date))
	if errors.Is(err, pgx.ErrNoRows) {
		return Day{}, ErrNoDay
	}

	if err != nil {
		return Day{}, fmt.Errorf("read the day %s: %w", date, err)
	}

	return d, nil
}

// RecordFinish counts one finished game against date and returns the new
// totals. The returned GamesPlayed is the player's finishing rank. The day must
// already have a row; claim it first.
func (s *Store) RecordFinish(ctx context.Context, date string, won bool, guesses int) (Day, error) {
	query := `
		UPDATE keyfordle.days SET
			games_played  = games_played + 1,
			wins          = wins + CASE WHEN $2::boolean THEN 1 ELSE 0 END,
			losses        = losses + CASE WHEN $2::boolean THEN 0 ELSE 1 END,
			total_guesses = total_guesses + $3::integer,
			updated_at    = now()
		WHERE date = $1::date
		RETURNING ` + dayColumns

	d, err := scanDay(s.pool.QueryRow(ctx, query, date, won, guesses))
	if errors.Is(err, pgx.ErrNoRows) {
		return Day{}, ErrNoDay
	}

	if err != nil {
		return Day{}, fmt.Errorf("record a finished game on %s: %w", date, err)
	}

	return d, nil
}

// PopularDays returns up to limit days with finished games, most played first.
// Ties go to the later day.
func (s *Store) PopularDays(ctx context.Context, limit int) ([]Day, error) {
	query := `SELECT ` + dayColumns + ` FROM keyfordle.days
		WHERE games_played > 0
		ORDER BY games_played DESC, date DESC
		LIMIT $1::integer`

	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("read the popular days: %w", err)
	}
	defer rows.Close()

	days := []Day{}

	for rows.Next() {
		d, err := scanDay(rows)
		if err != nil {
			return nil, fmt.Errorf("read a popular day: %w", err)
		}

		days = append(days, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the popular days: %w", err)
	}

	return days, nil
}

func scanDay(row pgx.Row) (Day, error) {
	var d Day

	err := row.Scan(
		&d.Date,
		&d.CardName,
		&d.GamesPlayed,
		&d.Wins,
		&d.Losses,
		&d.TotalGuesses,
		&d.CreatedAt,
		&d.UpdatedAt,
	)

	return d, err
}
