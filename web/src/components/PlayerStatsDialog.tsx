import { useEffect } from "react";
import { X } from "lucide-react";
import { type PlayerStats } from "../pages/keyFordleLogic";

interface PlayerStatsDialogProps {
  stats: PlayerStats;
  // highlight is the guess count of the shown day's win, to mark its bar.
  highlight?: number;
  onClose: () => void;
}

// PlayerStatsDialog shows the player's own results, read from the games saved
// in this browser.
const PlayerStatsDialog = ({ stats, highlight, onClose }: PlayerStatsDialogProps) => {
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const most = Math.max(1, ...stats.distribution);
  const numbers = [
    { label: "Played", value: stats.played },
    { label: "Win %", value: Math.round(stats.winRate) },
    { label: "Current streak", value: stats.currentStreak },
    { label: "Max streak", value: stats.maxStreak },
  ];

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Your stats"
        className="relative w-full max-w-sm rounded-lg border border-gray-600 bg-gray-800 p-5 text-white shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <button
          onClick={onClose}
          aria-label="Close"
          className="absolute right-3 top-3 rounded p-1 text-gray-400 hover:bg-gray-700 hover:text-white"
        >
          <X className="h-4 w-4" />
        </button>
        <h3 className="mb-4 text-center text-lg font-bold">Your stats</h3>

        <div className="mb-5 grid grid-cols-4 gap-2 text-center">
          {numbers.map((n) => (
            <div key={n.label}>
              <div className="text-2xl font-bold text-primary">{n.value}</div>
              <div className="text-xs text-gray-400">{n.label}</div>
            </div>
          ))}
        </div>

        <h4 className="mb-2 text-sm font-semibold">Guesses per win</h4>
        <div className="space-y-1">
          {stats.distribution.map((count, i) => (
            <div key={i} className="flex items-center gap-2 text-sm">
              <span className="w-3 text-right font-mono">{i + 1}</span>
              <div className="flex-1">
                <div
                  className={`min-w-[1.75rem] rounded px-2 text-right font-bold ${
                    highlight === i + 1 ? "bg-green-700" : "bg-gray-600"
                  }`}
                  style={{ width: `${(count / most) * 100}%` }}
                >
                  {count}
                </div>
              </div>
            </div>
          ))}
        </div>

        <p className="mt-4 text-center text-xs text-gray-400">
          Saved in this browser. A day found after Continue? counts as missed.
        </p>
      </div>
    </div>
  );
};

export default PlayerStatsDialog;
