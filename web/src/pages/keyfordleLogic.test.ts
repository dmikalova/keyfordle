import { describe, expect, it } from "vite-plus/test";
import { type Card, compareCards, formatRank, EXPANSIONS } from "./keyfordleLogic";

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
