import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig, type Plugin } from "vite";
import path from "path";
import { getDevSecurityHeaders } from "./src/lib/security";

const PROCESS_START = Date.now();

/**
 * Tiny inline Vite plugin that:
 *   1. Sets the recommended security headers on every dev response (CSP,
 *      X-Content-Type-Options, Referrer-Policy, Permissions-Policy).
 *   2. Serves the operator health endpoint at `/healthz` AND `/api/health`
 *      so curl works against either path.
 *
 * Kept inline so we don't need an extra package to wire up a few headers
 * and a JSON route in dev. Production builds run behind the INFINEON Go
 * server which sets the prod headers via `getProdSecurityHeaders()`.
 */
function infineonDevSecurityPlugin(): Plugin {
  const headers = getDevSecurityHeaders();
  return {
    name: "infineon-dev-security",
    apply: "serve",

    configureServer(devServer) {
      devServer.middlewares.use((req, res, next) => {
        for (const [k, v] of Object.entries(headers)) {
          res.setHeader(k, v);
        }

        const url = req.url ?? "";
        if (url === "/healthz" || url.startsWith("/healthz?")) {
          const body = JSON.stringify({
            status: "ok",
            uptime: Math.round((Date.now() - PROCESS_START) / 1000),
            version: "1.0.0",
          });
          res.statusCode = 200;
          res.setHeader("content-type", "application/json");
          res.setHeader("cache-control", "no-store");
          res.end(body);
          return;
        }
        if (url === "/api/health" || url.startsWith("/api/health?")) {
          const body = JSON.stringify({
            status: "ok",
            uptime: Math.round((Date.now() - PROCESS_START) / 1000),
            version: "1.0.0",
          });
          res.statusCode = 200;
          res.setHeader("content-type", "application/json");
          res.setHeader("cache-control", "no-store");
          res.end(body);
          return;
        }

        next();
      });
    },
  };
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), infineonDevSecurityPlugin()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  server: {
    host: "0.0.0.0",
    port: 3000,
    headers: getDevSecurityHeaders(),
  },
  optimizeDeps: {
    include: ["react-is", "recharts", "convex", "convex/react", "convex/server"],
  },
});