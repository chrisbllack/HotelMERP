import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

/**
 * 7 themes the operator can switch between at runtime.
 * Each one is mapped to a `[data-theme="..."]` selector in index.css which
 * retunes the entire UI (brand colour ramp, accent / gold ramp, display
 * font, corner radius, and panel shadow) by reassigning CSS variables.
 */
export type ThemeId =
  | "emerald-gold"
  | "midnight-sapphire"
  | "saffron-royale"
  | "lagoon-pearl"
  | "jade-imperial"
  | "obsidian-noir"
  | "coral-sunset";

export interface ThemeMeta {
  id: ThemeId;
  name: string;
  /** Cultural / regional inspiration for the theme. */
  inspiredBy: string;
  /** Two-line description of the feel. */
  tagline: string;
  /** Hex strings used for the theme switcher swatches. */
  swatch: { primary: string; accent: string; surface: string };
  /** Whether the theme is considered "dark" — used for preview chrome. */
  dark: boolean;
}

export const THEMES: ThemeMeta[] = [
  {
    id: "emerald-gold",
    name: "Emerald Gold",
    inspiredBy: "African luxury · Grand Palm signature",
    tagline: "Deep rainforest emerald with brass-gold accents. Warm, regal, grounded.",
    swatch: { primary: "#1f5d4b", accent: "#c9a227", surface: "#081a15" },
    dark: true,
  },
  {
    id: "midnight-sapphire",
    name: "Midnight Sapphire",
    inspiredBy: "United States · Four Seasons, Park Hyatt",
    tagline: "Cool navy with platinum highlights. Quiet, refined, unmistakably American luxury.",
    swatch: { primary: "#1a2a55", accent: "#b8c2d9", surface: "#050816" },
    dark: true,
  },
  {
    id: "saffron-royale",
    name: "Saffron Royale",
    inspiredBy: "India · Taj, Oberoi, Leela",
    tagline: "Deep maroon ground with royal saffron. Warm heritage, plush ceremony.",
    swatch: { primary: "#5c2b1a", accent: "#d4a017", surface: "#1a0a0a" },
    dark: true,
  },
  {
    id: "lagoon-pearl",
    name: "Lagoon Pearl",
    inspiredBy: "Nigeria · Eko Atlantic, coastal resorts",
    tagline: "Atlantic teal with pearl-champagne. Bright, breezy, distinctly coastal.",
    swatch: { primary: "#1f6e80", accent: "#d4b896", surface: "#051e26" },
    dark: true,
  },
  {
    id: "jade-imperial",
    name: "Jade Imperial",
    inspiredBy: "China · Mandarin Oriental, Peninsula",
    tagline: "Jadeite green with vermillion red. Serene, structured, ceremonial.",
    swatch: { primary: "#1f4d3a", accent: "#c4302b", surface: "#050d09" },
    dark: true,
  },
  {
    id: "obsidian-noir",
    name: "Obsidian Noir",
    inspiredBy: "Modern boutique · W Hotel, Ace",
    tagline: "Pure black with electric cyan. Bold, contemporary, deliberately sharp.",
    swatch: { primary: "#1f1f1f", accent: "#00d4ff", surface: "#000000" },
    dark: true,
  },
  {
    id: "coral-sunset",
    name: "Coral Sunset",
    inspiredBy: "Tropical resort · Bawah, Soneva",
    tagline: "Coral with apricot glow. Soft, generous, vacation in colour.",
    swatch: { primary: "#82323a", accent: "#e89a5f", surface: "#2a0e10" },
    dark: true,
  },
];

const STORAGE_KEY = "infineon.theme";

interface ThemeContextValue {
  theme: ThemeId;
  meta: ThemeMeta;
  setTheme: (id: ThemeId) => void;
  cycle: () => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

function readInitialTheme(): ThemeId {
  if (typeof window === "undefined") return "emerald-gold";
  const stored = window.localStorage.getItem(STORAGE_KEY) as ThemeId | null;
  if (stored && THEMES.some((t) => t.id === stored)) return stored;
  return "emerald-gold";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeId>(readInitialTheme);

  useEffect(() => {
    const root = document.documentElement;
    root.setAttribute("data-theme", theme);
    try {
      window.localStorage.setItem(STORAGE_KEY, theme);
    } catch {
      /* ignore quota errors */
    }
    // Hint the browser chrome for matching dark / light scheme
    root.style.colorScheme = "dark";
  }, [theme]);

  const setTheme = useCallback((id: ThemeId) => setThemeState(id), []);
  const cycle = useCallback(() => {
    setThemeState((prev) => {
      const idx = THEMES.findIndex((t) => t.id === prev);
      return THEMES[(idx + 1) % THEMES.length]!.id;
    });
  }, []);

  const value = useMemo<ThemeContextValue>(() => {
    const meta = THEMES.find((t) => t.id === theme) ?? THEMES[0]!;
    return { theme, meta, setTheme, cycle };
  }, [theme, setTheme, cycle]);

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    // Fall back to default theme if used outside provider (e.g. tests)
    return {
      theme: "emerald-gold",
      meta: THEMES[0]!,
      setTheme: () => undefined,
      cycle: () => undefined,
    };
  }
  return ctx;
}
