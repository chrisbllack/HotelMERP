# INFINEON Enterprise Platform — Wiki Index

This wiki is the project's persistent, compounding memory. It is consulted
before re-reading source code, and updated whenever a meaningful decision
or entity is added.

## Project
- [Overview](overview.md) — what INFINEON is, who it serves, where it lives
- [Team structure](team.md) — 10 parallel lanes that own the work
- [Visual themes](themes.md) — the 7 switchable UI themes
- [Grand Palm guest experience](grand-palm-guest-experience.md) — the rebrand of public surfaces to a luxury hospitality brand

## Modules (production scope)
- Front Desk, Reservations, Housekeeping, Maintenance, POS, Inventory, Finance,
  HR, CRM, Channels, Payments, Events, Hardware, Reports, Admin
- Public booking website: `/`, `/book`, `/book/guests`, `/booking/confirmation`, `/booking/lookup`

## Conventions
- Vite + React 19 + TypeScript + Tailwind v4
- Convex backend (`src/convex/`) with stub `_generated/` for sandbox preview
- Theme-aware styling via CSS variables — never hardcode brand colours outside `index.css`
