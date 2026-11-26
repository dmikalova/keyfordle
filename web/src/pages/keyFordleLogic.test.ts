import { describe, expect, it } from "vite-plus/test";
import {
  type Card,
  type DayResult,
  type DayStatus,
  addDays,
  compareCards,
  dayResultOf,
  formatRank,
  playerStats,
  EXPANSIONS,
} from "./keyFordleLogic";

const card = (over: Partial<Card>): Card => ({
  name: "Ancient Bear",
  house: "Untamed",
  card_type: "Creature",
  rarity: "Common",
  amber: 0,
  power: "5",
  armor: "0",
  expansion: 341,
  ...over,
});

describe("compareCards", () => {
  it("marks an identical card exact everywhere", () => {
    const target = card({});
    const r = compareCards(card({}), target);

    expect(r.houseMatch).toBe("exact");
    expect(r.typeMatch).toBe("exact");
    expect(r.rarityMatch).toBe("exact");
    expect(r.amberMatch).toBe("exact");
    expect(r.powerMatch).toBe("exact");
    expect(r.armorMatch).toBe("exact");
    expect(r.expansionMatch).toBe("exact");
    expect(r.nameMatches.every((m) => m === "correct")).toBe(true);
  });

  it("colors name letters the Wordle way, counting repeated letters", () => {
    // "aab" against "abc": a correct, second a absent (only one a), b present.
    const r = compareCards(card({ name: "aab" }), card({ name: "abc" }));

    expect(r.nameMatches).toEqual(["correct", "absent", "present"]);
  });

  it("orders rarity and reports the direction of the guess", () => {
    expect(compareCards(card({ rarity: "Rare" }), card({ rarity: "Common" })).rarityMatch).toBe(
      "more_rare",
    );
    expect(compareCards(card({ rarity: "Common" }), card({ rarity: "Special" })).rarityMatch).toBe(
      "less_rare",
    );
    expect(compareCards(card({ rarity: "Mythic" }), card({ rarity: "Common" })).rarityMatch).toBe(
      "none",
    );
  });

  it("compares numbers up and down", () => {
    const r = compareCards(
      card({ amber: 2, power: "3", armor: "1" }),
      card({ amber: 1, power: "6", armor: "1" }),
    );

    expect(r.amberMatch).toBe("higher");
    expect(r.powerMatch).toBe("lower");
    expect(r.armorMatch).toBe("exact");
  });

  it("treats no power as equal to no power and unlike a number", () => {
    const action = card({ card_type: "Action", power: undefined, armor: undefined });

    expect(compareCards(action, { ...action, name: "Other" }).powerMatch).toBe("exact");
    expect(compareCards(action, card({})).powerMatch).toBe("none");
  });

  it("reports an earlier or later expansion by Master Vault id", () => {
    expect(compareCards(card({ expansion: 341 }), card({ expansion: 928 })).expansionMatch).toBe(
      "earlier",
    );
    expect(compareCards(card({ expansion: 964 }), card({ expansion: 928 })).expansionMatch).toBe(
      "later",
    );
  });
});

describe("formatRank", () => {
  it.each([
    [1, "1st"],
    [2, "2nd"],
    [3, "3rd"],
    [4, "4th"],
    [11, "11th"],
    [12, "12th"],
    [13, "13th"],
    [21, "21st"],
    [112, "112th"],
  ])("formats %i as %s", (rank, want) => {
    expect(formatRank(rank)).toBe(want);
  });
});

describe("EXPANSIONS", () => {
  it("names every set the pool draws from", () => {
    for (const id of [341, 435, 452, 479, 496, 600, 700, 800, 855, 874, 886, 892, 918, 928]) {
      expect(EXPANSIONS[id]).toBeTruthy();
    }
  });
});

describe("addDays", () => {
  it("moves across months, years and leap days", () => {
    expect(addDays("2018-11-15", -1)).toBe("2018-11-14");
    expect(addDays("2018-12-31", 1)).toBe("2019-01-01");
    expect(addDays("2020-02-28", 1)).toBe("2020-02-29");
    expect(addDays("2020-03-01", -1)).toBe("2020-02-29");
  });
});

describe("dayResultOf", () => {
  const saved = (guesses: number, gameWon: boolean) =>
    JSON.stringify({ guesses: new Array(guesses).fill({}), gameWon });

  it("classifies a saved game", () => {
    expect(dayResultOf("2020-01-01", saved(3, true))?.status).toBe("won");
    expect(dayResultOf("2020-01-01", saved(6, false))?.status).toBe("lost");
    expect(dayResultOf("2020-01-01", saved(2, false))?.status).toBe("playing");
  });

  it("counts a find after continuing as lost", () => {
    expect(dayResultOf("2020-01-01", saved(8, true))?.status).toBe("lost");
  });

  it("skips empty or broken games", () => {
    expect(dayResultOf("2020-01-01", saved(0, false))).toBeNull();
    expect(dayResultOf("2020-01-01", "not json")).toBeNull();
  });
});

describe("playerStats", () => {
  const r = (date: string, status: DayStatus, guesses: number): DayResult => ({
    date,
    status,
    guesses,
  });

  it("sums wins, streaks and the guess distribution", () => {
    const stats = playerStats(
      [
        r("2026-11-20", "won", 2),
        r("2026-11-21", "won", 3),
        r("2026-11-22", "won", 3),
        r("2026-11-23", "lost", 6),
        r("2026-11-24", "won", 4),
        r("2026-11-25", "won", 1),
        r("2026-11-26", "playing", 2),
      ],
      "2026-11-26",
    );

    expect(stats.played).toBe(6);
    expect(stats.wins).toBe(5);
    expect(stats.maxStreak).toBe(3);
    // Today is unfinished, so the streak runs back from yesterday.
    expect(stats.currentStreak).toBe(2);
    expect(stats.distribution).toEqual([1, 1, 2, 1, 0, 0]);
  });

  it("ends the current streak at a finished today", () => {
    const stats = playerStats(
      [r("2026-11-25", "won", 2), r("2026-11-26", "lost", 6)],
      "2026-11-26",
    );
    expect(stats.currentStreak).toBe(0);
    expect(stats.maxStreak).toBe(1);
  });

  // The page renders before the server names today. A throw here blanked it.
  it("has no current streak before today is known", () => {
    const stats = playerStats([r("2026-11-25", "won", 2)], "");
    expect(stats.currentStreak).toBe(0);
    expect(stats.maxStreak).toBe(1);
  });

  it("is all zeros with no games", () => {
    expect(playerStats([], "2026-11-26")).toMatchObject({ played: 0, winRate: 0, maxStreak: 0 });
  });
});
