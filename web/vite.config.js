import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: "dist",
    emptyOutDir: true
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": "http://localhost:8000",
      "/extract-text": "http://localhost:8000",
      "/login": "http://localhost:8000",
      "/logout": "http://localhost:8000",
      "/admin": "http://localhost:8000",
      "/health": "http://localhost:8000",
    }
  }
});
