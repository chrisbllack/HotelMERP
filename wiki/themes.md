---
type: concept
title: 7 switchable visual themes
scope: project
---

INFINEON ships 7 selectable UI themes, switched at runtime via a
`[data-theme="..."]` attribute on `<html>`. Switching is instant — every
Tailwind colour utility (`bg-brand-700`, `text-gold-400`, etc.) resolves
through CSS variables, so the whole app retunes at once.

## Themes

| ID | Name | Inspiration |
| --- | --- | --- |
| `emerald-gold` | Emerald Gold (default) | African luxury — Grand Palm signature |
| `midnight-sapphire` | Midnight Sapphire | US — Four Seasons, Park Hyatt |
| `saffron-royale` | Saffron Royale | India — Taj, Oberoi, Leela |
| `lagoon-pearl` | Lagoon Pearl | Nigeria — Eko Atlantic, coastal resorts |
| `jade-imperial` | Jade Imperial | China — Mandarin Oriental, Peninsula |
| `obsidian-noir` | Obsidian Noir | Modern boutique — W Hotel, Ace |
| `coral-sunset` | Coral Sunset | Tropical resort — Bawah, Soneva |

## How it works

- `src/store/theme.tsx` — `ThemeProvider` + `useTheme()` hook, persists to `localStorage` under `infineon.theme`.
- `src/index.css` — defines `--color-brand-XXX` and `--color-gold-XXX` per theme.
- `src/components/ThemeSwitcher.tsx` — UI for selecting themes (grid in Admin, strip in topbar).

## Adding a new theme

1. Append a `ThemeMeta` entry to `THEMES` in `src/store/theme.tsx`.
2. Add a matching `[data-theme="new-id"] { ... }` block in `src/index.css` that redefines the brand/gold CSS variables.
3. Optionally override `--font-display`, `--radius-mode`, and `--shadow-panel` for the new feel.
