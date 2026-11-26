import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import autoprefixer from "autoprefixer";
import tailwindcss from "tailwindcss";
import { defineConfig, lazyPlugins } from "vite-plus";
import tailwindConfig from "./tailwind.config.ts";

// The page lives in web/. The build goes into the Go package that embeds it.
// Content-hashed files go under static/, which the Go server caches for a year;
// web/public keeps its names (the page links /assets/Aember.png), so it must
// not share a directory with them.
export default defineConfig({
  root: fileURLToPath(new URL("./web", import.meta.url)),
  build: {
    outDir: fileURLToPath(new URL("./internal/site/assets/app", import.meta.url)),
    emptyOutDir: true,
    assetsDir: "static",
  },
  css: {
    postcss: {
      plugins: [tailwindcss(tailwindConfig), autoprefixer()],
    },
  },
  server: {
    // Run the Go server on 8080 (`go run ./cmd/keyfordle -secrets ...`). The
    // proxy keeps the Host header, so the server's cross-origin check sees the
    // page and the API on one origin, as in production.
    proxy: {
      "/api": { target: "http://localhost:8080", changeOrigin: false },
    },
  },
  fmt: {
    overrides: [
      // project-standards writes lefthook.jsonc without trailing commas; the default adds them.
      { files: ["*.jsonc"], options: { trailingComma: "none" } },
      // project-standards writes pnpm-workspace.yaml with single quotes; the default uses double.
      { files: ["pnpm-workspace.yaml"], options: { singleQuote: true } },
    ],
  },
  lint: {
    plugins: ["react", "typescript", "oxc"],
    rules: {
      "react/rules-of-hooks": "error",
      "react/only-export-components": ["warn", { allowConstantExport: true }],
      "vite-plus/prefer-vite-plus-imports": "error",
    },
    options: {
      typeAware: true,
      typeCheck: true,
    },
    jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
  },
  plugins: lazyPlugins(() => [react()]),
});
