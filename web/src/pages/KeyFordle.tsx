import { useState, useEffect, useRef } from "react";
import { Search, Check, ArrowUp, ArrowDown, Copy, CheckCircle } from "lucide-react";
import api from "../utils/api";
import { useSEO } from "../hooks/useSEO";
import {
  type Card,
  type GuessResult,
  MAX_GUESSES,
  EXPANSIONS,
  formatDateFromString,
  formatRank,
  compareCards,
} from "./keyFordleLogic";

// Shown in place of a card image that fails to load.
const IMAGE_MISSING = "/assets/image-missing.svg";

interface GameState {
  date: string;
  targetCard: Card | null;
  guesses: GuessResult[];
  gameWon: boolean;
  gameFinished: boolean;
  playerRank?: number;
  // The player kept guessing after MAX_GUESSES misses. The loss is already
  // recorded; later guesses only help the player find the card.
  continued?: boolean;
  // The player chose to see the answer after running out of guesses.
  revealed?: boolean;
}

const KeyFordle = () => {
  useSEO({
    title: "KeyFordle - Daily Card Guessing Game",
    description: "Guess the KeyForge card of the day! A Wordle-style game for KeyForge cards.",
    canonical: "https://keyfordle.keyforge.cards/",
  });

  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<Card[]>([]);
  const [selectedCard, setSelectedCard] = useState<Card | null>(null);
  const [gameState, setGameState] = useState<GameState>({
    date: "",
    targetCard: null,
    guesses: [],
    gameWon: false,
    gameFinished: false,
  });
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [flippedIndexes, setFlippedIndexes] = useState<Set<number>>(new Set());
  const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  // The dropdown fills the window below the search box. It is measured each
  // time results arrive, so it follows scrolling and resizing between searches.
  const [dropdownMaxHeight, setDropdownMaxHeight] = useState(240);
  useEffect(() => {
    const input = searchInputRef.current;
    if (!input || searchResults.length === 0) return;
    const below = window.innerHeight - input.getBoundingClientRect().bottom - 16;
    setDropdownMaxHeight(Math.max(240, below));
  }, [searchResults]);
  const [globalStats, setGlobalStats] = useState<{
    games_played: number;
    wins: number;
    losses: number;
    win_rate: number;
    average_guesses: number;
    card_of_the_day: string | null;
  } | null>(null);
  const [copied, setCopied] = useState(false);

  // Handle click outside to close dropdown
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node) &&
        searchInputRef.current &&
        !searchInputRef.current.contains(event.target as Node)
      ) {
        setSearchResults([]);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const loadGameState = (serverDate: string) => {
    try {
      const savedState = localStorage.getItem(`keyfordle_${serverDate}`);

      if (savedState) {
        const parsed = JSON.parse(savedState);
        // Ensure backwards compatibility - add nameMatches to old guesses if missing
        if (parsed.guesses && Array.isArray(parsed.guesses)) {
          parsed.guesses.forEach((guess: GuessResult) => {
            if (guess && guess.card && guess.card.name && !guess.nameMatches) {
              // Generate nameMatches for old guesses (will be recalculated on next comparison anyway)
              guess.nameMatches = new Array(guess.card.name.length).fill("absent");
            }
          });
        }
        // Merge saved state with current state (preserve date and targetCard from backend)
        setGameState((prev) => ({
          ...prev,
          ...parsed,
          date: prev.date, // Keep server date
          targetCard: prev.targetCard || parsed.targetCard, // Prefer server targetCard
        }));
        // Restore flipped indexes for already revealed guesses
        const flipped = new Set<number>();
        parsed.guesses.forEach((_: GuessResult, index: number) => {
          flipped.add(index);
        });
        setFlippedIndexes(flipped);
        setLoading(false);
      } else {
        // New day - gameState already has date and targetCard from fetchDailyCardAndDate
        setLoading(false);
      }
    } catch (error) {
      console.error("Error loading game state:", error);
      setLoading(false);
    }
  };

  const fetchGlobalStats = async () => {
    try {
      const response = await api.get("/api/decks/public/keyfordle_stats/");
      setGlobalStats({
        games_played: response.data.games_played || 0,
        wins: response.data.wins || 0,
        losses: response.data.losses || 0,
        win_rate: response.data.win_rate || 0,
        average_guesses: response.data.average_guesses || 0,
        card_of_the_day: response.data.card_of_the_day || null,
      });
    } catch (error) {
      console.error("Error fetching global stats:", error);
    }
  };

  const recordGameCompletion = async (cardName: string, gameWon: boolean, numGuesses: number) => {
    try {
      const response = await api.post("/api/decks/public/record_keyfordle_completion/", {
        card_name: cardName,
        game_won: gameWon,
        num_guesses: numGuesses,
      });
      // Store the player rank from the response and save to localStorage
      if (response.data.player_rank) {
        setGameState((prev) => {
          const updatedState = {
            ...prev,
            playerRank: response.data.player_rank,
          };
          // Save updated state with rank to localStorage
          if (prev.date) {
            try {
              localStorage.setItem(`keyfordle_${prev.date}`, JSON.stringify(updatedState));
            } catch (error) {
              console.error("Error saving game state with rank:", error);
            }
          }
          return updatedState;
        });
      }
      // Refresh stats after recording
      await fetchGlobalStats();
    } catch (error) {
      console.error("Error recording game completion:", error);
    }
  };

  const fetchDailyCardAndDate = async () => {
    try {
      const response = await api.get("/api/decks/public/daily_card/");
      const dailyCard = response.data.card;
      const serverDate = response.data.date; // Use date from backend

      setGameState((prev) => {
        // Always update date from server, and targetCard if not already set
        return {
          ...prev,
          date: serverDate,
          targetCard: prev.targetCard || dailyCard,
        };
      });

      // Load game state from localStorage using the server date
      loadGameState(serverDate);
    } catch (error) {
      console.error("Error fetching daily card:", error);
      // Fallback to local date if backend fails (shouldn't happen normally)
      const fallbackDate = new Date().toISOString().split("T")[0];
      setGameState((prev) => ({
        ...prev,
        date: prev.date || fallbackDate,
      }));
      loadGameState(fallbackDate);
    }
  };

  // Load daily card first to get the server date, then load game state. The
  // effect sits below the functions it calls, so they exist when it reads them.
  // Both functions catch their own errors, so the promise is not awaited.
  useEffect(() => {
    const initializeGame = async () => {
      await fetchDailyCardAndDate();
      await fetchGlobalStats();
    };
    void initializeGame();
  }, []);

  const searchCards = async (query: string) => {
    if (query.length < 2) {
      setSearchResults([]);
      return;
    }

    setSearching(true);
    try {
      const response = await api.get(
        `/api/decks/public/card_search/?search=${encodeURIComponent(query)}&limit=50`,
      );
      setSearchResults(response.data.results || []);
    } catch (error) {
      console.error("Error searching cards:", error);
      setSearchResults([]);
    } finally {
      setSearching(false);
    }
  };

  const handleSearchChange = (value: string) => {
    setSearchQuery(value);
    setSelectedCard(null);

    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    // searchCards catches its own errors, so the promise is not awaited.
    searchTimeoutRef.current = setTimeout(() => {
      void searchCards(value);
    }, 300);
  };

  const handleCardSelect = (card: Card) => {
    setSelectedCard(card);
    setSearchQuery(card.name);
    setSearchResults([]);
    searchInputRef.current?.blur();
  };

  const handleSubmitGuess = () => {
    if (!selectedCard || !gameState.targetCard || gameState.gameFinished) {
      return;
    }

    // Check if already guessed this card
    if (gameState.guesses.some((g) => g.card.name === selectedCard.name)) {
      alert("You already guessed this card!");
      return;
    }

    const result = compareCards(selectedCard, gameState.targetCard);
    const newGuesses = [...gameState.guesses, result];
    const gameWon = selectedCard.name === gameState.targetCard.name;
    const gameFinished = gameWon || (!gameState.continued && newGuesses.length >= MAX_GUESSES);
    const wasFinished = gameState.gameFinished;

    const newState: GameState = {
      ...gameState,
      guesses: newGuesses,
      gameWon,
      gameFinished,
    };

    setGameState(newState);

    // Save to localStorage using server date from gameState
    try {
      const serverDate = gameState.date;
      if (!serverDate) {
        console.error("No date available in gameState");
        return;
      }

      localStorage.setItem(`keyfordle_${serverDate}`, JSON.stringify(newState));

      // Record completion to database when game finishes (only once per completion)
      if (gameFinished && !wasFinished) {
        // Check if we already recorded this completion (to avoid double counting on refresh)
        const completedKey = `keyfordle_completed_${serverDate}`;
        if (!localStorage.getItem(completedKey)) {
          localStorage.setItem(completedKey, "true");
          // recordGameCompletion catches its own errors, so the promise is not awaited.
          void recordGameCompletion(newState.targetCard?.name || "", gameWon, newGuesses.length);
        }
      }
    } catch (error) {
      console.error("Error saving game state:", error);
    }

    // Flip animation
    const guessIndex = newGuesses.length - 1;
    setTimeout(() => {
      setFlippedIndexes((prev) => new Set(prev).add(guessIndex));
    }, 100);

    // Reset search
    setSelectedCard(null);
    setSearchQuery("");
  };

  // Saves a state change that is not a guess: continuing or revealing.
  const updateGameState = (changes: Partial<GameState>) => {
    const newState = { ...gameState, ...changes };
    setGameState(newState);

    try {
      if (newState.date) {
        localStorage.setItem(`keyfordle_${newState.date}`, JSON.stringify(newState));
      }
    } catch (error) {
      console.error("Error saving game state:", error);
    }
  };

  const handleContinue = () => updateGameState({ continued: true, gameFinished: false });

  const handleReveal = () => updateGameState({ gameFinished: true, revealed: true });

  // Search results without the cards already guessed.
  const visibleResults = searchResults.filter(
    (card) => !gameState.guesses.some((g) => g.card.name === card.name),
  );

  // The player missed MAX_GUESSES times and has not seen the answer yet.
  const outOfGuesses =
    !gameState.gameWon && !gameState.revealed && gameState.guesses.length >= MAX_GUESSES;

  // A continuing player gets one empty row below their guesses.
  const rowCount = gameState.continued
    ? Math.max(MAX_GUESSES, gameState.guesses.length + (gameState.gameFinished ? 0 : 1))
    : MAX_GUESSES;

  const generateShareMessage = (): string => {
    if (!gameState.targetCard || !gameState.date) return "";

    const dateStr = formatDateFromString(gameState.date, {
      month: "long",
      day: "numeric",
      year: "numeric",
    });
    const guessCount = gameState.guesses.length;
    const resultEmoji = gameState.gameWon ? "🎉" : "😔";
    const overTime = guessCount > MAX_GUESSES;
    const resultText = gameState.gameWon
      ? overTime
        ? "Solved after continuing"
        : "Solved"
      : "Failed";

    let message = `KeyFordle ${dateStr} ${resultText} ${resultEmoji}\n`;
    message += overTime
      ? `${guessCount} guesses (${MAX_GUESSES} allowed)\n`
      : `${guessCount}/${MAX_GUESSES} guesses\n`;

    // Add rank information if available
    if (gameState.playerRank) {
      const rankText = formatRank(gameState.playerRank);
      message += `I was the ${rankText} to finish!\n`;
    }

    message += `\n`;

    // Add emoji grid for each guess
    gameState.guesses.forEach((guess) => {
      // Attributes in order: House, Type, Rarity, Set, Amber, Power, Armor (matching frontend display)
      // Using ⬛ (black square) to represent red/wrong matches since there's no standard red square emoji
      const houseEmoji = guess.houseMatch === "exact" ? "🟩" : "⬛";
      const typeEmoji = guess.typeMatch === "exact" ? "🟩" : "⬛";
      const rarityEmoji =
        guess.rarityMatch === "exact"
          ? "🟩"
          : guess.rarityMatch === "more_rare"
            ? "🟨"
            : guess.rarityMatch === "less_rare"
              ? "🟨"
              : "⬛";
      const expEmoji =
        guess.expansionMatch === "exact"
          ? "🟩"
          : guess.expansionMatch === "earlier" || guess.expansionMatch === "later"
            ? "🟨"
            : "⬛";

      // Numeric attributes: Green for exact, Yellow for higher/lower, Black for none (red in UI)
      const amberEmoji =
        guess.amberMatch === "exact"
          ? "🟩"
          : guess.amberMatch === "higher" || guess.amberMatch === "lower"
            ? "🟨"
            : "⬛";
      const powerEmoji =
        guess.powerMatch === "exact"
          ? "🟩"
          : guess.powerMatch === "higher" || guess.powerMatch === "lower"
            ? "🟨"
            : "⬛";
      const armorEmoji =
        guess.armorMatch === "exact"
          ? "🟩"
          : guess.armorMatch === "higher" || guess.armorMatch === "lower"
            ? "🟨"
            : "⬛";

      message += `${houseEmoji}${typeEmoji}${rarityEmoji}${expEmoji}${amberEmoji}${powerEmoji}${armorEmoji}\n`;
    });

    message += `\nPlay at: https://keyfordle.keyforge.cards`;
    return message;
  };

  const handleCopyShare = async () => {
    try {
      const message = generateShareMessage();
      await navigator.clipboard.writeText(message);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (error) {
      console.error("Error copying to clipboard:", error);
    }
  };

  const getCellColor = (
    guess: GuessResult | null,
    attribute: "house" | "type" | "rarity" | "amber" | "power" | "armor" | "expansion",
    index: number,
  ) => {
    if (!guess || !flippedIndexes.has(index)) {
      return "bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600";
    }

    switch (attribute) {
      case "house":
        return guess.houseMatch === "exact"
          ? "bg-green-500 text-white border-green-600"
          : "bg-red-400 dark:bg-red-600 text-white border-red-500 dark:border-red-700";
      case "type":
        return guess.typeMatch === "exact"
          ? "bg-green-500 text-white border-green-600"
          : "bg-red-400 dark:bg-red-600 text-white border-red-500 dark:border-red-700";
      case "rarity":
        if (guess.rarityMatch === "exact") {
          return "bg-green-500 text-white border-green-600";
        } else if (guess.rarityMatch === "more_rare") {
          return "bg-yellow-500 text-white border-yellow-600";
        } else if (guess.rarityMatch === "less_rare") {
          return "bg-yellow-500 text-white border-yellow-600";
        }
        return "bg-red-400 dark:bg-red-600 text-white border-red-500 dark:border-red-700";
      case "amber":
      case "power":
      case "armor":
        let matchType: "exact" | "higher" | "lower" | "none";
        if (attribute === "amber") matchType = guess.amberMatch;
        else if (attribute === "power") matchType = guess.powerMatch;
        else matchType = guess.armorMatch;

        if (matchType === "exact") {
          return "bg-green-500 text-white border-green-600";
        } else if (matchType === "higher" || matchType === "lower") {
          return "bg-yellow-500 text-white border-yellow-600";
        }
        return "bg-red-400 dark:bg-red-600 text-white border-red-500 dark:border-red-700";
      case "expansion":
        if (guess.expansionMatch === "exact") {
          return "bg-green-500 text-white border-green-600";
        } else if (guess.expansionMatch === "earlier" || guess.expansionMatch === "later") {
          return "bg-yellow-500 text-white border-yellow-600";
        }
        return "bg-red-400 dark:bg-red-600 text-white border-red-500 dark:border-red-700";
      default:
        return "bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600";
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div className="text-xl text-gray-600 dark:text-gray-400">Loading...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 py-8">
      <div className="max-w-4xl mx-auto px-4">
        {/* Header */}
        <div className="text-center mb-8">
          <h1 className="text-4xl font-bold text-primary mb-2">KeyFordle</h1>
          <p className="text-gray-600 dark:text-gray-400">
            Guess the KeyForge card of the day! You have {MAX_GUESSES} tries.
          </p>
          {gameState.date && (
            <p className="text-sm text-gray-500 dark:text-gray-500 mt-2">
              {formatDateFromString(gameState.date)}
            </p>
          )}
        </div>

        {/* Global Statistics */}
        {globalStats && globalStats.games_played > 0 && (
          <div className="mb-6 p-4 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
            <h3 className="text-lg font-semibold mb-3 text-center text-gray-900 dark:text-white">
              Today's Statistics
            </h3>
            <div className="grid grid-cols-2 md:grid-cols-5 gap-4 text-center">
              <div>
                <div className="text-2xl font-bold text-gray-900 dark:text-white">
                  {globalStats.games_played}
                </div>
                <div className="text-sm text-gray-600 dark:text-gray-400">Games Played</div>
              </div>
              <div>
                <div className="text-2xl font-bold text-green-600 dark:text-green-400">
                  {globalStats.wins}
                </div>
                <div className="text-sm text-gray-600 dark:text-gray-400">Wins</div>
              </div>
              <div>
                <div className="text-2xl font-bold text-red-600 dark:text-red-400">
                  {globalStats.losses}
                </div>
                <div className="text-sm text-gray-600 dark:text-gray-400">Losses</div>
              </div>
              <div>
                <div className="text-2xl font-bold text-primary">
                  {globalStats.win_rate.toFixed(1)}%
                </div>
                <div className="text-sm text-gray-600 dark:text-gray-400">Win Rate</div>
              </div>
              <div>
                <div className="text-2xl font-bold text-purple-600 dark:text-purple-400">
                  {globalStats.average_guesses.toFixed(1)}
                </div>
                <div className="text-sm text-gray-600 dark:text-gray-400">Avg Guesses</div>
              </div>
            </div>
          </div>
        )}

        {/* Search Bar */}
        <div className="mb-6 relative">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-gray-400" />
            <input
              ref={searchInputRef}
              type="text"
              value={searchQuery}
              onChange={(e) => handleSearchChange(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && selectedCard) {
                  handleSubmitGuess();
                }
              }}
              placeholder="Search for a card..."
              disabled={gameState.gameFinished}
              className="w-full pl-10 pr-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50 disabled:cursor-not-allowed"
            />
            {searching && (
              <div className="absolute right-3 top-1/2 transform -translate-y-1/2">
                <div className="animate-spin h-5 w-5 border-2 border-primary border-t-transparent rounded-full"></div>
              </div>
            )}
          </div>

          {/* Dropdown Results */}
          {visibleResults.length > 0 && !selectedCard && (
            <div
              ref={dropdownRef}
              className="absolute z-50 top-full mt-1 w-full bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-lg shadow-lg overflow-y-auto"
              style={{ maxHeight: dropdownMaxHeight }}
            >
              {visibleResults.map((card) => (
                <button
                  key={card.name}
                  onClick={() => handleCardSelect(card)}
                  className="w-full text-left px-4 py-3 hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 last:border-b-0 transition-colors"
                >
                  <div className="font-medium text-base mb-1">{card.name}</div>
                  <div className="flex flex-wrap items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
                    {card.house && (
                      <span className="px-2 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 rounded">
                        {card.house}
                      </span>
                    )}
                    {card.card_type && (
                      <span className="px-2 py-0.5 bg-purple-100 dark:bg-purple-900 text-purple-800 dark:text-purple-200 rounded">
                        {card.card_type}
                      </span>
                    )}
                    {card.rarity && (
                      <span className="px-2 py-0.5 bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200 rounded">
                        {card.rarity}
                      </span>
                    )}
                    {card.expansion && (
                      <span className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded">
                        {EXPANSIONS[card.expansion] || `Set ${card.expansion}`}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-3 mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {card.amber !== undefined && (
                      <span className="flex items-center gap-1">
                        <img
                          src="/assets/Aember.png"
                          alt="amber"
                          className="w-3 h-3"
                          onError={(e) => {
                            e.currentTarget.style.display = "none";
                          }}
                        />
                        {card.amber}
                      </span>
                    )}
                    {card.power && card.power !== "-" && <span>⚔️ {card.power}</span>}
                    {card.armor && card.armor !== "-" && <span>🛡️ {card.armor}</span>}
                  </div>
                </button>
              ))}
            </div>
          )}

          {/* Submit Button */}
          <div className="mt-4 flex gap-4">
            <button
              onClick={handleSubmitGuess}
              disabled={!selectedCard || gameState.gameFinished}
              className="flex-1 bg-primary hover:bg-primary-dark disabled:bg-gray-400 disabled:cursor-not-allowed text-gray-900 font-medium py-2 px-4 rounded-lg transition-colors"
            >
              Submit Guess
            </button>
          </div>
        </div>

        {/* Out of guesses: keep going or see the answer */}
        {outOfGuesses && (
          <div className="mb-6 p-3 rounded-lg bg-gray-800 border border-gray-700 flex flex-wrap items-center justify-center gap-3 text-sm">
            <span className="text-gray-300">
              {gameState.continued
                ? "Still guessing. Your result counts as a miss."
                : `Out of guesses after ${MAX_GUESSES} tries.`}
            </span>
            {!gameState.continued && (
              <button
                onClick={handleContinue}
                className="px-4 py-1.5 rounded-lg bg-primary hover:bg-primary-dark text-gray-900 font-medium transition-colors"
              >
                Continue?
              </button>
            )}
            <button
              onClick={handleReveal}
              className="px-4 py-1.5 rounded-lg border border-gray-600 text-white hover:bg-gray-700 transition-colors"
            >
              Show answer
            </button>
          </div>
        )}

        {/* Game Result Message */}
        {gameState.gameFinished &&
          gameState.targetCard &&
          (gameState.gameWon || gameState.revealed) && (
            <div
              className={`mb-6 p-4 rounded-lg text-center ${
                gameState.gameWon
                  ? "bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200"
                  : "bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200"
              }`}
            >
              {gameState.gameWon ? (
                <div>
                  <p className="font-bold text-lg">Congratulations! You found it!</p>
                  <p className="mt-1">
                    The card was: <strong>{gameState.targetCard.name}</strong>
                  </p>
                  <p className="mt-2 text-sm">
                    {gameState.guesses.length > MAX_GUESSES
                      ? `Guessed in ${gameState.guesses.length} tries, after continuing past ${MAX_GUESSES}`
                      : `Guessed in ${gameState.guesses.length}/${MAX_GUESSES} tries`}
                  </p>
                </div>
              ) : (
                <div>
                  <p className="font-bold text-lg">Better luck tomorrow!</p>
                  <p className="mt-1">
                    The card was: <strong>{gameState.targetCard.name}</strong>
                  </p>
                  <p className="mt-2 text-sm">You used {gameState.guesses.length} guesses</p>
                </div>
              )}

              {/* Share Button */}
              <button
                onClick={handleCopyShare}
                className="mt-4 inline-flex items-center gap-2 px-4 py-2 bg-white dark:bg-gray-800 text-gray-900 dark:text-white rounded-lg border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
              >
                {copied ? (
                  <>
                    <CheckCircle className="h-4 w-4 text-green-600" />
                    Copied!
                  </>
                ) : (
                  <>
                    <Copy className="h-4 w-4" />
                    Share Result
                  </>
                )}
              </button>
            </div>
          )}

        {/* Guess Grid */}
        <div className="space-y-3">
          {Array.from({ length: rowCount }).map((_, index) => {
            const guess = gameState.guesses[index] || null;
            const isFlipped = flippedIndexes.has(index);
            // Use the same image field logic - CardSerializer returns 'image' field with the correct URL
            const cardImage = guess?.card?.image;

            return (
              <div key={index} className="flex gap-3 items-center">
                {/* Card Image */}
                {guess && cardImage ? (
                  <div className="flex-shrink-0 flex flex-col items-center">
                    <img
                      src={cardImage}
                      alt={guess.card.name}
                      className="w-20 h-28 object-contain drop-shadow-md transition-transform duration-300 hover:scale-[5] hover:z-50 relative"
                      onError={(e) => {
                        // A card whose image is not uploaded yet shows the
                        // placeholder. The check stops a loop if the
                        // placeholder fails too.
                        if (!e.currentTarget.src.endsWith(IMAGE_MISSING)) {
                          e.currentTarget.src = IMAGE_MISSING;
                        }
                      }}
                    />
                  </div>
                ) : (
                  <div className="w-20 h-28 rounded border border-gray-200 dark:border-gray-600 bg-gray-100 dark:bg-gray-800 flex-shrink-0 flex items-center justify-center">
                    <span className="text-gray-400 dark:text-gray-600 text-xs">—</span>
                  </div>
                )}

                {/* Card Name and Attributes Row */}
                <div className="flex-1 flex flex-col gap-2">
                  {/* Card Name - Wordle-style letter coloring */}
                  <div className="p-2 rounded-lg border-2 border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800">
                    <div className="font-semibold text-left text-sm">
                      {guess && isFlipped ? (
                        <div className="flex items-center justify-start gap-0.5 flex-wrap">
                          {guess.card.name.split("").map((letter, letterIndex) => {
                            // Handle backwards compatibility - nameMatches might not exist for old saved games
                            const matchStatus =
                              (guess.nameMatches && guess.nameMatches[letterIndex]) || "absent";
                            let bgColor = "bg-gray-400 dark:bg-gray-600";
                            if (matchStatus === "correct") {
                              bgColor = "bg-green-500 dark:bg-green-600";
                            } else if (matchStatus === "present") {
                              bgColor = "bg-yellow-500 dark:bg-yellow-600";
                            } else {
                              bgColor = "bg-red-400 dark:bg-red-600";
                            }

                            return (
                              // Every tile has the same width, so letter N
                              // sits in the same column in every guess. The
                              // hover label is CSS, not a title attribute,
                              // so it shows at once.
                              <span
                                key={letterIndex}
                                aria-label={`${letter === " " ? "space" : letter}, character ${letterIndex + 1}`}
                                className={`group relative inline-block w-5 py-0.5 text-center font-mono rounded text-white font-bold cursor-default ${bgColor} transition-colors duration-300`}
                                style={{
                                  animationDelay: `${letterIndex * 50}ms`,
                                  animation: isFlipped ? "flip 0.6s ease-in-out" : "none",
                                }}
                              >
                                {letter === " " ? "\u00A0" : letter}
                                <span
                                  aria-hidden="true"
                                  className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1 hidden -translate-x-1/2 rounded bg-gray-950 px-1 text-[10px] leading-4 text-white group-hover:block"
                                >
                                  {letterIndex + 1}
                                </span>
                              </span>
                            );
                          })}
                        </div>
                      ) : (
                        <span className="text-gray-900 dark:text-white">
                          {guess ? guess.card.name : "—"}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* All Attributes in One Row - Flex with equal widths, scrollable on small screens */}
                  <div className="flex gap-2 overflow-x-auto pb-1">
                    {/* House */}
                    <div
                      className={`px-2 py-2 rounded-lg border-2 text-center font-medium transition-all duration-500 flex-1 min-w-[70px] ${getCellColor(
                        guess,
                        "house",
                        index,
                      )} ${isFlipped && guess ? "" : "text-gray-900 dark:text-white"}`}
                    >
                      <div
                        className={`text-xs mb-1 ${isFlipped && guess ? "text-white/80" : "text-gray-500 dark:text-gray-400"}`}
                      >
                        House
                      </div>
                      <div className="flex items-center justify-center gap-1 text-xs">
                        {guess ? guess.card.house || "—" : "—"}
                        {isFlipped && guess && guess.houseMatch === "exact" && (
                          <Check className="h-3 w-3" />
                        )}
                      </div>
                    </div>

                    {/* Type */}
                    <div
                      className={`px-2 py-2 rounded-lg border-2 text-center font-medium transition-all duration-500 flex-1 min-w-[70px] ${getCellColor(
                        guess,
                        "type",
                        index,
                      )} ${isFlipped && guess ? "" : "text-gray-900 dark:text-white"}`}
                    >
                      <div
                        className={`text-xs mb-1 ${isFlipped && guess ? "text-white/80" : "text-gray-500 dark:text-gray-400"}`}
                      >
                        Type
                      </div>
                      <div className="flex items-center justify-center gap-1 text-xs">
                        {guess ? guess.card.card_type || "—" : "—"}
                        {isFlipped && guess && guess.typeMatch === "exact" && (
                          <Check className="h-3 w-3" />
                        )}
                      </div>
                    </div>

                    {/* Rarity */}
                    <div
                      className={`px-2 py-2 rounded-lg border-2 text-center font-medium transition-all duration-500 flex-1 min-w-[70px] ${getCellColor(
                        guess,
                        "rarity",
                        index,
                      )} ${isFlipped && guess ? "" : "text-gray-900 dark:text-white"}`}
                    >
                      <div
                        className={`text-xs mb-1 ${isFlipped && guess ? "text-white/80" : "text-gray-500 dark:text-gray-400"}`}
                      >
                        Rarity
                      </div>
                      <div className="flex items-center justify-center gap-1 text-xs">
                        {guess ? guess.card.rarity || "—" : "—"}
                        {isFlipped && guess && guess.rarityMatch === "exact" && (
                          <Check className="h-3 w-3" />
                        )}
                        {isFlipped && guess && guess.rarityMatch === "more_rare" && (
                          <ArrowUp className="h-3 w-3" />
                        )}
                        {isFlipped && guess && guess.rarityMatch === "less_rare" && (
                          <ArrowDown className="h-3 w-3" />
                        )}
                      </div>
                    </div>

                    {/* Set */}
                    <div
                      className={`px-2 py-2 rounded-lg border-2 text-center font-medium transition-all duration-500 flex-1 min-w-[70px] ${getCellColor(
                        guess,
                        "expansion",
                        index,
                      )} ${isFlipped && guess ? "" : "text-gray-900 dark:text-white"}`}
                    >
                      <div
                        className={`text-xs mb-1 ${isFlipped && guess ? "text-white/80" : "text-gray-500 dark:text-gray-400"}`}
                      >
                        Set
                      </div>
                      <div className="flex items-center justify-center gap-1 text-xs">
                        {guess && guess.card.expansion
                          ? EXPANSIONS[guess.card.expansion] || `${guess.card.expansion}`
                          : "—"}
                        {isFlipped && guess && guess.expansionMatch === "exact" && (
                          <Check className="h-3 w-3" />
                        )}
                        {isFlipped && guess && guess.expansionMatch === "earlier" && (
                          <ArrowDown className="h-3 w-3" />
                        )}
                        {isFlipped && guess && guess.expansionMatch === "later" && (
                          <ArrowUp className="h-3 w-3" />
                        )}
                      </div>
                    </div>

                    {/* Amber */}
                    <div
                      className={`px-2 py-2 rounded-lg border-2 text-center font-medium transition-all duration-500 flex-1 min-w-[70px] ${getCellColor(
                        guess,
                        "amber",
                        index,
                      )} ${isFlipped && guess ? "" : "text-gray-900 dark:text-white"}`}
                    >
                      <div
                        className={`text-xs mb-1 ${isFlipped && guess ? "text-white/80" : "text-gray-500 dark:text-gray-400"}`}
                      >
                        Amber
                      </div>
                      <div className="flex items-center justify-center gap-1 text-xs">
                        {guess ? guess.card.amber || 0 : "—"}
                        {isFlipped && guess && guess.amberMatch === "exact" && (
                          <Check className="h-3 w-3" />
                        )}
                        {isFlipped && guess && guess.amberMatch === "higher" && (
                          <ArrowUp className="h-3 w-3" />
                        )}
                        {isFlipped && guess && guess.amberMatch === "lower" && (
                          <ArrowDown className="h-3 w-3" />
                        )}
                      </div>
                    </div>

                    {/* Power */}
                    <div
                      className={`px-2 py-2 rounded-lg border-2 text-center font-medium transition-all duration-500 flex-1 min-w-[70px] ${getCellColor(
                        guess,
                        "power",
                        index,
                      )} ${isFlipped && guess ? "" : "text-gray-900 dark:text-white"}`}
                    >
                      <div
                        className={`text-xs mb-1 ${isFlipped && guess ? "text-white/80" : "text-gray-500 dark:text-gray-400"}`}
                      >
                        Power
                      </div>
                      <div className="flex items-center justify-center gap-1 text-xs">
                        {guess ? guess.card.power || "—" : "—"}
                        {isFlipped && guess && guess.powerMatch === "exact" && (
                          <Check className="h-3 w-3" />
                        )}
                        {isFlipped && guess && guess.powerMatch === "higher" && (
                          <ArrowUp className="h-3 w-3" />
                        )}
                        {isFlipped && guess && guess.powerMatch === "lower" && (
                          <ArrowDown className="h-3 w-3" />
                        )}
                      </div>
                    </div>

                    {/* Armor */}
                    <div
                      className={`px-2 py-2 rounded-lg border-2 text-center font-medium transition-all duration-500 flex-1 min-w-[70px] ${getCellColor(
                        guess,
                        "armor",
                        index,
                      )} ${isFlipped && guess ? "" : "text-gray-900 dark:text-white"}`}
                    >
                      <div
                        className={`text-xs mb-1 ${isFlipped && guess ? "text-white/80" : "text-gray-500 dark:text-gray-400"}`}
                      >
                        Armor
                      </div>
                      <div className="flex items-center justify-center gap-1 text-xs">
                        {guess ? guess.card.armor || "—" : "—"}
                        {isFlipped && guess && guess.armorMatch === "exact" && (
                          <Check className="h-3 w-3" />
                        )}
                        {isFlipped && guess && guess.armorMatch === "higher" && (
                          <ArrowUp className="h-3 w-3" />
                        )}
                        {isFlipped && guess && guess.armorMatch === "lower" && (
                          <ArrowDown className="h-3 w-3" />
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        {/* Legend */}
        <div className="mt-8 p-4 bg-gray-100 dark:bg-gray-800 rounded-lg">
          <h3 className="font-bold text-white mb-2">How to play</h3>
          <p className="text-sm text-gray-300 mb-3">
            Find today&apos;s card in {MAX_GUESSES} guesses. Each guess shows how{" "}
            <strong>your guess</strong> compares to today&apos;s card.
          </p>
          <ul className="space-y-2 text-sm text-gray-300">
            <li className="flex items-start gap-2">
              <span className="inline-flex h-5 w-5 flex-shrink-0 items-center justify-center rounded bg-green-500 text-white">
                <Check className="h-3 w-3" />
              </span>
              <span>Same as today&apos;s card.</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="inline-flex h-5 w-5 flex-shrink-0 items-center justify-center rounded bg-yellow-500 text-white">
                <ArrowUp className="h-3 w-3" />
              </span>
              <span>
                Close. Your guess is higher, rarer, or from a newer set. Today&apos;s card is lower,
                less rare, or older.
              </span>
            </li>
            <li className="flex items-start gap-2">
              <span className="inline-flex h-5 w-5 flex-shrink-0 items-center justify-center rounded bg-yellow-500 text-white">
                <ArrowDown className="h-3 w-3" />
              </span>
              <span>
                Close. Your guess is lower, less rare, or from an older set. Today&apos;s card is
                higher, rarer, or newer.
              </span>
            </li>
            <li className="flex items-start gap-2">
              <span className="inline-block h-5 w-5 flex-shrink-0 rounded bg-red-500" />
              <span>Different from today&apos;s card.</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="inline-flex flex-shrink-0 gap-0.5">
                <span className="w-5 rounded bg-green-500 text-center font-mono font-bold text-white">
                  A
                </span>
                <span className="w-5 rounded bg-yellow-500 text-center font-mono font-bold text-white">
                  B
                </span>
                <span className="w-5 rounded bg-red-500 text-center font-mono font-bold text-white">
                  C
                </span>
              </span>
              <span>
                Name letters: green is in the right spot, yellow is elsewhere in the name, red is
                not in the name.
              </span>
            </li>
            <li>
              Out of guesses? Choose <strong>Continue?</strong> to keep guessing. The day still
              counts as a miss.
            </li>
          </ul>
        </div>
      </div>
    </div>
  );
};

export default KeyFordle;
