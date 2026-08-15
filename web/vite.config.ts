import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

// The demo build is served from a subpath on GitHub Pages
// (https://<user>.github.io/UniDisk/), so assets need a matching base. Override
// with VITE_BASE when deploying elsewhere. Normal builds stay at the root,
// since the Go server serves the SPA from /.
export default defineConfig({
  base: process.env.VITE_BASE ?? (process.env.VITE_DEMO === "1" ? "/UniDisk/" : "/"),
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    // Proxy API calls to the Go backend during development so the SPA and API
    // share an origin from the browser's perspective.
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
