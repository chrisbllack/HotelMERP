/**
 * Security headers & Content Security Policy for the INFINEON console.
 *
 * The single source of truth for the recommended HTTP response headers the
 * web tier should emit. `vite.config.ts` consumes `getDevSecurityHeaders()`
 * for the Vite dev server; the production server (Go binary embedding the
 * built `dist/`) should set the same keys verbatim. Keep them in sync.
 *
 * Why each header matters here:
 *   - CSP locks down script/style/img/frame permissions and explicitly allows
 *     the Vite HMR websocket (`ws:`/`wss:`) and the Convex placeholder
 *     domain so the booking surface still connects when VITE_CONVEX_URL
 *     is unset.
 *   - X-Content-Type-Options: nosniff stops MIME sniffing attacks.
 *   - Referrer-Policy: strict-origin-when-cross-origin keeps the path off
 *     outbound links.
 *   - Permissions-Policy disables every sensor we don't actually use so a
 *     compromised script can't quietly enable the camera or mic.
 *   - X-Frame-Options is intentionally OMITTED so the ProductOS preview
 *     iframe (and any future embed) can render the app. `frame-ancestors *`
 *     in the CSP carries the same intent in spec-compliant browsers; legacy
 *     IE/Edge are not in our support matrix.
 *   - HSTS is gated on production only — `includeSubDomains; preload` over
 *     the dev connection would brick local `.localhost` subdomains.
 */

export interface SecurityHeaders {
  [key: string]: string;
}

const CONVEX_PLACEHOLDER_HOST = "https://convex-not-configured.invalid";

/**
 * Returns the recommended response headers for the Vite **dev** server.
 * Inline scripts/styles are allowed because Vite's HMR runtime injects
 * them at runtime; production builds use hashed module scripts instead.
 */
export function getDevSecurityHeaders(): SecurityHeaders {
  const csp = [
    "default-src 'self'",
    // Vite HMR client connects to ws://localhost:port; allow ws/wss broadly.
    `connect-src 'self' ws: wss: https://*.convex.cloud ${CONVEX_PLACEHOLDER_HOST} http: https:`,
    // Vite injects an inline module bootstrap script + React refresh helpers.
    "script-src 'self' 'unsafe-inline' 'unsafe-eval'",
    // Tailwind v4 ships a single inline `<style>` with the design tokens.
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob: https:",
    "font-src 'self' data:",
    // Allow the ProductOS preview iframe (and any other embed).
    "frame-ancestors *",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ].join("; ");

  return {
    "Content-Security-Policy": csp,
    "X-Content-Type-Options": "nosniff",
    "Referrer-Policy": "strict-origin-when-cross-origin",
    "Permissions-Policy": [
      "camera=()",
      "microphone=()",
      "geolocation=()",
      "payment=()",
      "usb=()",
      "magnetometer=()",
      "gyroscope=()",
      "accelerometer=()",
    ].join(", "),
    // Deliberately no X-Frame-Options — see file comment.
    "Cross-Origin-Opener-Policy": "same-origin-allow-popups",
  };
}

/**
 * Recommended headers for **production** (Go static server / CDN). Inline
 * scripts are disallowed because Vite emits hashed module bundles; HSTS is
 * added because the connection is always TLS in prod.
 */
export function getProdSecurityHeaders(): SecurityHeaders {
  const csp = [
    "default-src 'self'",
    `connect-src 'self' https://*.convex.cloud ${CONVEX_PLACEHOLDER_HOST} https:`,
    "script-src 'self'",
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob: https:",
    "font-src 'self' data:",
    "frame-ancestors *",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "upgrade-insecure-requests",
  ].join("; ");

  return {
    "Content-Security-Policy": csp,
    "X-Content-Type-Options": "nosniff",
    "Referrer-Policy": "strict-origin-when-cross-origin",
    "Permissions-Policy": [
      "camera=()",
      "microphone=()",
      "geolocation=()",
      "payment=()",
      "usb=()",
      "magnetometer=()",
      "gyroscope=()",
      "accelerometer=()",
    ].join(", "),
    "Strict-Transport-Security": "max-age=63072000; includeSubDomains; preload",
    "Cross-Origin-Opener-Policy": "same-origin-allow-popups",
  };
}