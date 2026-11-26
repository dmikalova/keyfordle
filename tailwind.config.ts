import { fileURLToPath } from "node:url";
import type { Config } from "tailwindcss";

const web = (path: string) => fileURLToPath(new URL(`./web/${path}`, import.meta.url));

// Upstream KFStats' Tailwind 3 config, cut to what the Keyfordle page uses,
// with the keyforge.cards theme. keyforge.cards is dark only, so index.html
// sets the dark class and the page's dark: styles always apply.
//
// The theme replaces the gray scale with keyforge.cards' purples, so the
// page's classes keep their upstream names: gray-900 is the page surface,
// gray-800 a card, gray-400 muted text. primary is the orange accent. Green,
// yellow, red and blue stay Tailwind's, because the page uses them for guess
// results.
export default {
  content: [web("index.html"), web("src/**/*.{ts,tsx}")],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        gray: {
          50: "#faf8fb",
          100: "#f3eff5",
          200: "#e8e2ec",
          300: "#d6cddd",
          400: "#bcafc6",
          500: "#8a7798",
          600: "#6a5578",
          700: "#4f3a5b",
          800: "#3d2b47",
          900: "#2b1d33",
          950: "#1f1425",
        },
        primary: {
          DEFAULT: "#f5a623",
          dark: "#d48806",
        },
      },
      fontFamily: {
        sans: [
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "Roboto",
          "Oxygen",
          "Ubuntu",
          "sans-serif",
        ],
      },
    },
  },
  plugins: [],
} satisfies Config;
