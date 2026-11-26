import { fileURLToPath } from "node:url";
import type { Config } from "tailwindcss";

const web = (path: string) => fileURLToPath(new URL(`./web/${path}`, import.meta.url));

// Upstream KFStats' Tailwind 3 config, cut to what the Keyfordle page uses.
// Dark mode follows the system: the KFStats dark-mode toggle is not lifted.
export default {
  content: [web("index.html"), web("src/**/*.{ts,tsx}")],
  darkMode: "media",
  theme: {
    extend: {
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
      },
    },
  },
  plugins: [],
} satisfies Config;
