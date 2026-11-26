# keyfordle

KeyFordle, the daily KeyForge card-guessing game, at
[keyfordle.keyforge.cards](https://keyfordle.keyforge.cards).

KeyFordle was created by the KFStats contributors
([Valnor280/KFStats](https://github.com/Valnor280/KFStats)). This repository
lifts its game page as is and serves it from a Go backend. See [NOTICE](NOTICE).

## How it works

1. The first request of a UTC day picks the day's card from the card pool and
   stores it in Postgres.
2. The page searches the pool as the player types and compares each guess with
   the day's card in the browser.
3. A finished game adds to the day's totals. The page shows the totals and the
   player's finishing rank.

The card pool is `internal/cards/cards.json`, embedded in the binary. Card
images come from the public keyforge.cards image bucket.

## Cards left out

The pool holds the actions, artifacts, creatures and upgrades of player decks.
`mage cards` records every other card in
[docs/excluded-cards.json](docs/excluded-cards.json), with the reason.
[docs/excluded-cards.md](docs/excluded-cards.md) gives each reason, its card
count, and what it takes to add those cards.

## Routes

| Route                                                 | Purpose                  |
| ----------------------------------------------------- | ------------------------ |
| `GET /api/decks/public/daily_card/`                   | The day's card and date  |
| `GET /api/decks/public/card_search/?search=&limit=`   | Cards whose name matches |
| `POST /api/decks/public/record_keyfordle_completion/` | Count a finished game    |
| `GET /api/decks/public/keyfordle_stats/`              | The day's totals         |
| `GET /health`                                         | Liveness, no database    |
| `GET /*`                                              | The page                 |

The paths are the upstream KFStats paths, so the page calls them unchanged.

## Developing

```sh
vp install                   # frontend dependencies
mage ci:fix && mage ci:check # Go gate, must print ALL GREEN
vp check && vp test          # frontend gate
vp run build                 # build the page into internal/site/assets/app
```

Run it against a local Postgres. Apply `db/schema.hcl` with Atlas first, then
write a config file:

```sh
echo '{"DATABASE_URL_TRANSACTION": "postgres://..."}' > tmp/config.json
go run ./cmd/keyfordle -secrets tmp/config.json
vp dev                       # page with hot reload, API proxied to :8080
```

The `internal/store` tests run against that database when
`KEYFORDLE_TEST_DATABASE_URL` names it, and skip otherwise.

Regenerate the card pool from a vex checkout after vex updates its catalogs:

```sh
mage cards -vex=../vex
```

## License

Apache-2.0, with portions under the MIT License. See [LICENSE](LICENSE) and
[NOTICE](NOTICE).
