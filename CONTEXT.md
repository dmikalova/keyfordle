# Context

The vocabulary of this repository.

## Card pool

The cards a player can guess and the day can pick: every action, artifact,
creature and upgrade of a player deck in vex's set catalogs, one per name.

## Excluded card

A catalog card the pool leaves out, with its reason: gigantic creatures (both
halves), evil twins, token creatures, archon powers, prophecies, the tide and
the Keyraken adventure deck. `docs/excluded-cards.json` lists them.

## First printing

The set a pool card comes from: the first set in release order that printed
it. Its house, rarity, expansion and image come from that set.

## Day

One UTC date. A day has one card and the totals of the games finished on it.
The first request of the day stores the card.

## Daily pick

The rule that chooses a new day's card, kept from upstream: the MD5 of the
date as `YYYYMMDD`, read as an integer, modulo the pool size, indexes the pool
in name order.

## Finished game

A game the player won or lost after six guesses. The page reports it once, and
the server adds it to the day's totals.

## Rank

The order in which a player finished among the day's finished games. It is the
day's game count right after the player's game is added.

## Config secret

The single JSON object the `cloud-run-app` module mounts at
`/secrets/config.json`. It holds `DATABASE_URL_TRANSACTION` for the app and
`DATABASE_URL_SESSION` for CI's schema apply.
