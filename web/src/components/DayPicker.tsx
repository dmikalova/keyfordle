import { useEffect, useRef, useState } from "react";
import { CalendarDays, ChevronLeft, ChevronRight } from "lucide-react";
import { type DayStatus, formatDateFromString } from "../pages/keyFordleLogic";

const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

const WEEKDAYS = ["S", "M", "T", "W", "T", "F", "S"];

// The color of a day the player has played. A finished day is filled; a day in
// progress is outlined.
const STATUS_CLASS: Record<DayStatus, string> = {
  won: "bg-green-700 text-white",
  lost: "bg-red-700 text-white",
  playing: "ring-2 ring-inset ring-yellow-600",
};

interface DayPickerProps {
  value: string;
  min: string;
  max: string;
  results: Map<string, DayStatus>;
  onSelect: (date: string) => void;
}

// ymd writes a UTC year, zero-based month and day as YYYY-MM-DD.
const ymd = (year: number, month: number, day: number) =>
  new Date(Date.UTC(year, month, day)).toISOString().slice(0, 10);

// DayPicker is a month calendar in the keyforge.cards theme. Changing the month
// or year keeps it open; picking a day closes it. Days the player has played
// show their result.
const DayPicker = ({ value, min, max, results, onSelect }: DayPickerProps) => {
  const [open, setOpen] = useState(false);
  const [view, setView] = useState({ year: 2018, month: 10 });
  const rootRef = useRef<HTMLDivElement>(null);

  const [minYear, minMonth] = min.split("-").map(Number);
  const [maxYear, maxMonth] = (max || value).split("-").map(Number);

  // Close on a click outside or on Escape.
  useEffect(() => {
    if (!open) return;
    const onMouseDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onMouseDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onMouseDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const toggle = () => {
    if (!open) {
      const [y, m] = value.split("-").map(Number);
      setView({ year: y, month: m - 1 });
    }
    setOpen(!open);
  };

  // A month is shown only if some of its days fall between min and max.
  const monthIndex = (year: number, month: number) => year * 12 + month;
  const firstMonth = monthIndex(minYear, minMonth - 1);
  const lastMonth = monthIndex(maxYear, maxMonth - 1);
  const clampView = (year: number, month: number) => {
    const i = Math.min(lastMonth, Math.max(firstMonth, monthIndex(year, month)));
    setView({ year: Math.floor(i / 12), month: i % 12 });
  };
  const current = monthIndex(view.year, view.month);

  const startWeekday = new Date(Date.UTC(view.year, view.month, 1)).getUTCDay();
  const daysInMonth = new Date(Date.UTC(view.year, view.month + 1, 0)).getUTCDate();
  const years = Array.from({ length: maxYear - minYear + 1 }, (_, i) => minYear + i);

  return (
    <div ref={rootRef} className="relative">
      <button
        onClick={toggle}
        aria-haspopup="dialog"
        aria-expanded={open}
        className="flex items-center gap-2 px-3 py-1 rounded-lg border border-gray-600 bg-gray-800 text-white hover:bg-gray-700"
      >
        <CalendarDays className="h-4 w-4 text-primary" />
        {formatDateFromString(value, { month: "short", day: "numeric", year: "numeric" })}
      </button>

      {open && (
        <div
          role="dialog"
          aria-label="Pick a day"
          className="absolute left-1/2 top-full z-50 mt-2 w-72 -translate-x-1/2 rounded-lg border border-gray-600 bg-gray-800 p-3 text-white shadow-xl"
        >
          <div className="mb-2 flex items-center justify-between gap-1">
            <button
              onClick={() => clampView(view.year, view.month - 1)}
              disabled={current <= firstMonth}
              aria-label="Previous month"
              className="rounded p-1 hover:bg-gray-700 disabled:opacity-30"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <select
              value={view.month}
              onChange={(e) => clampView(view.year, Number(e.target.value))}
              aria-label="Month"
              className="rounded bg-gray-900 px-1 py-0.5 text-sm"
            >
              {MONTHS.map((name, i) => (
                <option
                  key={name}
                  value={i}
                  disabled={
                    monthIndex(view.year, i) < firstMonth || monthIndex(view.year, i) > lastMonth
                  }
                >
                  {name}
                </option>
              ))}
            </select>
            <select
              value={view.year}
              onChange={(e) => clampView(Number(e.target.value), view.month)}
              aria-label="Year"
              className="rounded bg-gray-900 px-1 py-0.5 text-sm"
            >
              {years.map((y) => (
                <option key={y} value={y}>
                  {y}
                </option>
              ))}
            </select>
            <button
              onClick={() => clampView(view.year, view.month + 1)}
              disabled={current >= lastMonth}
              aria-label="Next month"
              className="rounded p-1 hover:bg-gray-700 disabled:opacity-30"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>

          <div className="grid grid-cols-7 gap-1 text-center text-xs">
            {WEEKDAYS.map((d, i) => (
              <div key={i} className="py-1 text-gray-400">
                {d}
              </div>
            ))}
            {Array.from({ length: startWeekday }, (_, i) => (
              <div key={`pad-${i}`} />
            ))}
            {Array.from({ length: daysInMonth }, (_, i) => {
              const date = ymd(view.year, view.month, i + 1);
              const status = results.get(date);
              const outside = date < min || date > max;
              const selected = date === value;
              return (
                <button
                  key={date}
                  onClick={() => {
                    onSelect(date);
                    setOpen(false);
                  }}
                  disabled={outside}
                  aria-current={selected ? "date" : undefined}
                  className={`h-8 rounded text-sm transition-colors disabled:opacity-25 disabled:cursor-not-allowed ${
                    status ? STATUS_CLASS[status] : "hover:bg-gray-700"
                  } ${selected ? "outline outline-2 outline-primary" : ""}`}
                >
                  {i + 1}
                </button>
              );
            })}
          </div>

          <div className="mt-3 flex items-center justify-center gap-3 text-xs text-gray-300">
            <span className="flex items-center gap-1">
              <span className="h-3 w-3 rounded-sm bg-green-700" /> Won
            </span>
            <span className="flex items-center gap-1">
              <span className="h-3 w-3 rounded-sm bg-red-700" /> Missed
            </span>
            <span className="flex items-center gap-1">
              <span className="h-3 w-3 rounded-sm ring-2 ring-inset ring-yellow-600" /> Started
            </span>
          </div>
        </div>
      )}
    </div>
  );
};

export default DayPicker;
