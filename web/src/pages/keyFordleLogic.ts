// KeyFordle's rules, moved out of KeyFordle.tsx unchanged so they can be tested.
// The comparison is upstream's compareCards; only its formatting changed.

export interface Card {
  name: string;
  name_en?: string;
  house?: string;
  card_type?: string;
  rarity?: string;
  amber: number;
  power?: string;
  armor?: string;
  expansion?: number;
  image_url?: string;
  image?: string;
}

export interface GuessResult {
  card: Card;
  nameMatches: ("correct" | "present" | "absent")[]; // Letter-by-letter match status for card name
  houseMatch: "exact" | "none";
  typeMatch: "exact" | "none";
  rarityMatch: "exact" | "more_rare" | "less_rare" | "none";
  amberMatch: "exact" | "higher" | "lower" | "none";
  powerMatch: "exact" | "higher" | "lower" | "none";
  armorMatch: "exact" | "higher" | "lower" | "none";
  expansionMatch: "exact" | "earlier" | "later" | "none";
}

export const MAX_GUESSES = 6;

export const EXPANSIONS: Record<number, string> = {
  341: "CotA",
  435: "AoA",
  452: "WC",
  907: "D",
  479: "MM",
  496: "DT",
  600: "WoE",
  601: "UC22",
  609: "VM23",
  700: "GR",
  722: "M",
  737: "VM24",
  800: "AS",
  855: "ToC",
  874: "MMM",
  939: "VM25",
  886: "PV",
  918: "CC",
  // Sets upstream's map lacked. Their cards are in the keyforge.cards pool.
  892: "MCW",
  928: "DM",
  964: "VM26",
};

// Helper function to format date string as local date (not UTC)
export const formatDateFromString = (dateString: string, options?: Intl.DateTimeFormatOptions) => {
  // Parse YYYY-MM-DD as local date, not UTC
  const [year, month, day] = dateString.split("-").map(Number);
  const date = new Date(year, month - 1, day); // month is 0-indexed in JS Date
  return date.toLocaleDateString(
    "en-US",
    options || {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric",
    },
  );
};

// Helper function to format rank (1st, 2nd, 3rd, etc.)
export const formatRank = (rank: number): string => {
  const lastDigit = rank % 10;
  const lastTwoDigits = rank % 100;

  if (lastTwoDigits >= 11 && lastTwoDigits <= 13) {
    return `${rank}th`;
  }

  switch (lastDigit) {
    case 1:
      return `${rank}st`;
    case 2:
      return `${rank}nd`;
    case 3:
      return `${rank}rd`;
    default:
      return `${rank}th`;
  }
};

export const compareCards = (guessed: Card, target: Card): GuessResult => {
  // Name comparison - Wordle-style letter matching
  const compareNames = (
    guessedName: string,
    targetName: string,
  ): ("correct" | "present" | "absent")[] => {
    const guessed = guessedName.toLowerCase().split("");
    const target = targetName.toLowerCase().split("");
    const result: ("correct" | "present" | "absent")[] = new Array(guessed.length).fill("absent");

    // Count all letters in target word
    const targetLetterCounts = new Map<string, number>();
    target.forEach((letter) => {
      targetLetterCounts.set(letter, (targetLetterCounts.get(letter) || 0) + 1);
    });

    // First pass: mark correct positions (green) and decrement counts
    guessed.forEach((letter, i) => {
      if (i < target.length && letter === target[i]) {
        result[i] = "correct";
        targetLetterCounts.set(letter, (targetLetterCounts.get(letter) || 0) - 1);
      }
    });

    // Second pass: mark present letters in wrong positions (yellow)
    guessed.forEach((letter, i) => {
      if (result[i] === "correct") return; // Skip already marked as correct

      // Check if this letter is still available in the target (after accounting for correct positions)
      const count = targetLetterCounts.get(letter) || 0;
      if (count > 0) {
        result[i] = "present";
        targetLetterCounts.set(letter, count - 1);
      }
    });

    return result;
  };

  const nameMatches = compareNames(guessed.name, target.name);

  // House comparison - exact match
  const houseMatch: "exact" | "none" = guessed.house === target.house ? "exact" : "none";

  // Type comparison - exact match
  const typeMatch: "exact" | "none" = guessed.card_type === target.card_type ? "exact" : "none";

  // Rarity comparison - exact match, more rare, or less rare
  let rarityMatch: "exact" | "more_rare" | "less_rare" | "none" = "none";
  if (guessed.rarity && target.rarity) {
    if (guessed.rarity === target.rarity) {
      rarityMatch = "exact";
    } else {
      // Define rarity hierarchy (lower number = less rare)
      const rarityOrder: { [key: string]: number } = {
        Common: 1,
        Uncommon: 2,
        Rare: 3,
        Special: 4,
        Variant: 5,
        "Evil Twin": 5,
      };

      const guessedRarityOrder = rarityOrder[guessed.rarity] || 0;
      const targetRarityOrder = rarityOrder[target.rarity] || 0;

      if (guessedRarityOrder > 0 && targetRarityOrder > 0) {
        if (guessedRarityOrder > targetRarityOrder) {
          rarityMatch = "more_rare";
        } else if (guessedRarityOrder < targetRarityOrder) {
          rarityMatch = "less_rare";
        } else {
          rarityMatch = "none";
        }
      } else {
        rarityMatch = "none";
      }
    }
  }

  // Amber comparison - exact match or higher/lower
  let amberMatch: "exact" | "higher" | "lower" | "none" = "none";
  const guessedAmber = guessed.amber || 0;
  const targetAmber = target.amber || 0;

  if (guessedAmber === targetAmber) {
    amberMatch = "exact";
  } else if (guessedAmber > targetAmber) {
    amberMatch = "higher";
  } else if (guessedAmber < targetAmber) {
    amberMatch = "lower";
  }

  // Power comparison - exact match or higher/lower
  let powerMatch: "exact" | "higher" | "lower" | "none" = "none";
  const parsePower = (val: string | undefined): number | "X" | null => {
    if (!val || val === "-" || val === "") return null;
    if (val.toLowerCase() === "x") return "X";
    const parsed = parseInt(val, 10);
    return isNaN(parsed) ? null : parsed;
  };
  const guessedPower = parsePower(guessed.power);
  const targetPower = parsePower(target.power);

  // If both are null/empty, it's an exact match (both have no power)
  if (guessedPower === null && targetPower === null) {
    powerMatch = "exact";
  } else if (guessedPower === "X" && targetPower === "X") {
    // Both are "X" - exact match
    powerMatch = "exact";
  } else if (typeof guessedPower === "number" && typeof targetPower === "number") {
    // Both are numbers - compare numerically
    if (guessedPower === targetPower) {
      powerMatch = "exact";
    } else if (guessedPower > targetPower) {
      powerMatch = "higher";
    } else if (guessedPower < targetPower) {
      powerMatch = "lower";
    }
  }
  // Otherwise (X vs null, X vs number, null vs number, etc.) - no match

  // Armor comparison - exact match or higher/lower
  let armorMatch: "exact" | "higher" | "lower" | "none" = "none";
  const parseArmor = (val: string | undefined): number | "X" | null => {
    if (!val || val === "-" || val === "") return null;
    if (val.toLowerCase() === "x") return "X";
    const parsed = parseInt(val, 10);
    return isNaN(parsed) ? null : parsed;
  };
  const guessedArmor = parseArmor(guessed.armor);
  const targetArmor = parseArmor(target.armor);

  // If both are null/empty, it's an exact match (both have no armor)
  if (guessedArmor === null && targetArmor === null) {
    armorMatch = "exact";
  } else if (guessedArmor === "X" && targetArmor === "X") {
    // Both are "X" - exact match
    armorMatch = "exact";
  } else if (typeof guessedArmor === "number" && typeof targetArmor === "number") {
    // Both are numbers - compare numerically
    if (guessedArmor === targetArmor) {
      armorMatch = "exact";
    } else if (guessedArmor > targetArmor) {
      armorMatch = "higher";
    } else if (guessedArmor < targetArmor) {
      armorMatch = "lower";
    }
  }
  // Otherwise (X vs null, X vs number, null vs number, etc.) - no match

  // Expansion comparison - exact match, earlier, or later
  let expansionMatch: "exact" | "earlier" | "later" | "none" = "none";
  if (guessed.expansion && target.expansion) {
    if (guessed.expansion === target.expansion) {
      expansionMatch = "exact";
    } else if (guessed.expansion < target.expansion) {
      expansionMatch = "earlier";
    } else if (guessed.expansion > target.expansion) {
      expansionMatch = "later";
    }
  }

  return {
    card: guessed,
    nameMatches,
    houseMatch,
    typeMatch,
    rarityMatch,
    amberMatch,
    powerMatch,
    armorMatch,
    expansionMatch,
  };
};

// FIRST_DAY is KeyForge's release, the earliest day a player can pick. The
// server sends the same day; this is the fallback before it answers.
export const FIRST_DAY = "2018-11-15";

// addDays moves a YYYY-MM-DD day by n days, in UTC like the server.
export function addDays(date: string, n: number): string {
  const [y, m, d] = date.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, d + n)).toISOString().slice(0, 10);
}
