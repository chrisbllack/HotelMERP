import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ConvexProvider, ConvexReactClient } from "convex/react";
import "./index.css";
import App from "./App";
import { ThemeProvider } from "./store/theme";

// Convex powers the integrated booking website and live console panels.
// When VITE_CONVEX_URL is absent the client points at a placeholder so the
// app still renders; booking features show their "connecting" state.
const convexAddress = import.meta.env.VITE_CONVEX_URL as string | undefined;
const convexClient = new ConvexReactClient(convexAddress ?? "https://convex-not-configured.invalid");

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider>
      <ConvexProvider client={convexClient}>
        <App />
      </ConvexProvider>
    </ThemeProvider>
  </StrictMode>,
);
