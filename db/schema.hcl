schema "keyfordle" {}

// One row per UTC day: the day's card and the running totals of finished games.
// The first request of the day inserts the row, so the day keeps its card even
// if a later deploy changes the card pool.
table "days" {
  schema = schema.keyfordle

  column "date" {
    type = date
    null = false
  }

  // The card's name, as the pool in internal/cards spells it.
  column "card_name" {
    type = text
    null = false
  }

  column "games_played" {
    type    = integer
    null    = false
    default = 0
  }

  column "wins" {
    type    = integer
    null    = false
    default = 0
  }

  column "losses" {
    type    = integer
    null    = false
    default = 0
  }

  // The sum of the guess counts of every finished game.
  column "total_guesses" {
    type    = integer
    null    = false
    default = 0
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  column "updated_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.date]
  }

  check "days_counts_non_negative" {
    expr = "games_played >= 0 AND wins >= 0 AND losses >= 0 AND total_guesses >= 0"
  }

  check "days_outcomes_add_up" {
    expr = "wins + losses = games_played"
  }
}
