import path from "path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { monaco } from "@bithero/monaco-editor-vite-plugin";

export default defineConfig({
  plugins: [
    react(),
    monaco({
      features: "all",
      languages: ["yaml", "javascript"],
      globalAPI: true,
    }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    chunkSizeWarningLimit: 1600,
  },
  base: "",
});
