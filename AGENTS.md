# keyfordle

Conventions for this repository.

## Validation

Run both gates before calling work done:

- `mage ci:fix && mage ci:check` must print `ALL GREEN`.
- `vp check && vp test && vp run build` must pass. CI runs the same.

## Package layout

| Path               | Role                                                   |
| ------------------ | ------------------------------------------------------ |
| `cmd/keyfordle`    | The Cloud Run entrypoint: load, connect, serve         |
| `internal/cards`   | The embedded card pool: lookup, search, the daily pick |
| `internal/catalog` | Building the pool from keyforge-data (`mage cards`)    |
| `internal/config`  | Reading `/secrets/config.json` and validating it       |
| `internal/server`  | The API routes and the cross-origin check              |
| `internal/site`    | Serving the built page with its cache headers          |
| `internal/store`   | The Postgres queries                                   |
| `web/`             | The React page lifted from KFStats                     |
| `db/schema.hcl`    | The Atlas schema that CI applies to Supabase           |

## Rules

- **The page is a lift and shift.** `web/src/pages/KeyFordle.tsx` stays as
  close to upstream KFStats as the toolchain allows. A change to the game goes
  in its own commit and names the upstream behavior it changes. The planned
  replacement is Go with templ and Datastar, not a rewrite in React.
- **The API keeps the upstream paths and JSON shapes.** The page calls them
  unchanged. Do not rename a route or a field without changing the page in the
  same commit.
- **The database decides who wins.** The day's card is claimed with an insert
  that does nothing on conflict, then read back. A finished game is one
  `UPDATE ... RETURNING`. Do not replace either with a read followed by a write.
- **`/health` never touches the database.**
- **Never hand-edit `internal/cards/cards.json` or `docs/excluded-cards.*`.**
  Run `mage cards`. It writes the pool and the excluded list from the
  keyforge-data dataset, and formats them with `vp fmt`. It reads vex's
  catalogs only for the sets that the dataset does not have yet. Run it again
  after a dataset publish adds a set. Every card lands in exactly one of the
  two; a new kind of card to leave out gets a `Reason`, a
  section in `internal/catalog/doc.go`, and a case in the catalog tests.
  Card names keep Master Vault's spelling, typos included, so the spell check
  skips the generated `internal/cards/cards.json` and `docs/excluded-cards.json`
  (`tools.misspell.exclude` in `mklv.config.json`).
- **Image paths follow vex's importer:** `<set-slug>/<number><ext>`, or
  `<number>-<name-slug><ext>` when two cards in one set share a number, under
  `images/cards/` in the `mklv-keyforge-cards-public` bucket. The base URL and
  the extension are `ImageBaseURL` and `ImageExt` in `internal/catalog`;
  change them there and run `mage cards`. `mage images <dir>` converts
  the importer's PNGs to WebP at quality 85 and uploads them; the PNG
  originals go to the private `mklv-keyforge-cards` bucket. A card whose
  image fails to load shows `web/public/assets/image-missing.svg`.

## Talking to Supabase

The connection string is the transaction pooler on port 6543. That pooler
hands a different backend to each statement, so prepared statements do not
survive. `internal/store` sets `pgx.QueryExecModeSimpleProtocol` and zeroes
both caches. Do not remove those lines.

Migrations run in CI, not at boot. CI finds `db/schema.hcl`, reads
`DATABASE_URL_SESSION` from the `keyfordle-config` secret and runs
`atlas schema apply` against the `keyfordle` schema. It refuses to drop.
