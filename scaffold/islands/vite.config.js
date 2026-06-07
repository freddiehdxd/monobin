import { defineConfig } from "vite";
import preact from "@preact/preset-vite";
import tailwindcss from "@tailwindcss/vite";

// Builds island bundles into ../app/assets with STABLE names (entry.js,
// style.css) so the Go side can embed and reference them without a manifest.
export default defineConfig({
  plugins: [preact(), tailwindcss()],
  build: {
    outDir: "../app/assets",
    emptyOutDir: true,
    rollupOptions: {
      input: "src/entry.js",
      output: {
        entryFileNames: "entry.js",
        chunkFileNames: "[name].js",
        assetFileNames: (info) => {
          const name = info.name || (info.names && info.names[0]) || "";
          return name.endsWith(".css") ? "style.css" : "[name][extname]";
        },
      },
    },
  },
});
