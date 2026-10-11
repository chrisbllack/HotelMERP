---
name: platform-marketing-site
description: B2B operator-facing marketing site at /platform — synthesizes patterns from cloudhotelerp / djubo / yanolja
metadata:
  type: entity
---

# /platform — INFINEON Enterprise Platform marketing site

A public B2B surface for the INFINEON Enterprise Platform, distinct from the
guest-facing Grand Palm Hotel Lagos rebrand. Operators (GMs, owners, IT
decision-makers) hit `/platform` from the Grand Palm nav badge or directly,
and find a sales-grade overview of the platform: 21 modules, 4 suites,
ecosystem diagram, integrations, comparison table, pricing, FAQ, and CTA.

The route was added in October 2026 after studying four reference sites:

| Site | What we borrowed |
| --- | --- |
| [cloudhotelerp.com](https://cloudhotelerp.com/) | "One stop solution" hero promise, social-proof chips, vertical section rhythm |
| [cloudhotelerp.com/hotel-management-software.html](https://cloudhotelerp.com/hotel-management-software.html) | Module ecosystem diagram, suite grouping, brochure-style module icons |
| [cloudhotelerp.com/brochure.html](https://cloudhotelerp.com/brochure.html) | "Download the brochure" content depth, brand-blue + gold palette idea |
| [djubo.com](https://www.djubo.com/) | "Smarter hotel. Higher revenue." editorial voice, demo CTA, 2500+ hotels social proof chip |
| [djubo.com/products](https://www.djubo.com/products) | Suite split (5 Distribution / 11 Operations / 4 Intelligence), product card pattern (icon + name + tagline + learn-more link) |
| [djubo.com/pricing](https://www.djubo.com/pricing) | Three-tier pricing, monthly/annual toggle, "most popular" highlight |
| [yanolja.com](https://www.yanolja.com/) | Site was under construction; only the brand voice was useful |

## Page anatomy (top → bottom)

1. **Header** — INFINEON logomark, suite navigation, "See a live property →" (deep links to Grand Palm), Sign in, gold "Request demo" CTA
2. **Hero** — Full-bleed Unsplash image (operator at a console), 88vh tall, dual gradient overlay, eyebrow "Hotel management software · 21 modules", H1 with gold-gradient accent on the keyword phrase
3. **Social proof strip** — "240+ properties · 19 countries · Used in Lagos, Mumbai, Manila, Mexico City, Cairo"
4. **Modules** — 21 cards in a 3-col grid with All/Operations/Distribution/Intelligence/Guest filter chips; each card has a gold icon, name, tagline, suite badge, "Open module →" deep link
5. **Suite narrative** — 4 narrative cards restating the suite split
6. **Ecosystem diagram** — 17 color-coded nodes (sky = in, gold = core, brand = out, emerald = audit, rose = guest) showing how channels/booking flow into PMS and out to housekeeping, F&B, audit, etc. Inspired by cloudhotelerp's branching tree
7. **Integrations** — OTA/payment/accounting/hardware tag cloud + 6 capability cards (SAML SSO, open data, webhooks, PCI-aware, API/SDK, queues)
8. **Comparison** — INFINEON vs three "Cloud competitor" placeholders, on 7 capabilities. (Naming kept generic; switch to real names when/if a public benchmark is published.)
9. **Testimonials** — 3 operator quotes with check-mark decoration
10. **Pricing** — 3 tiers (Boutique, Grand, Estate) with Grand highlighted as "Most popular", Monthly/Annual toggle, Naira pricing
11. **FAQ** — 5 collapsible questions (the first opens by default)
12. **Final CTA** — Full-bleed image overlay, "Run the property in 14 days. Decide in 30." with gold "Request a 14-day trial" + "Talk to sales" buttons
13. **Footer** — 4-column with product/resources/contact links

## Source files

- `src/pages/platform/PlatformPage.tsx` — the whole page
- `src/App.tsx` — public route registration: `<Route path="/platform" element={<PlatformPage />} />`
- `src/pages/LandingPage.tsx` — added a "Run your hotel on this →" badge in the nav to drive operators to the platform

## Design language

- Same CSS-variable palette as the rest of the app (`brand-950`, `gold-400/500`, `emerald-400`, `rose-400`, `sky-400`). No new theme.
- Section-vertical rhythm alternates `brand-950` and `brand-900/30`.
- Headlines use the project display face; section eyebrows use 10px uppercase tracking-[0.25em] gold.
- Filter chips are gold-on-deep-when-active, brand-on-deep-when-inactive.
- Decorative icons use `lucide-react`. No brand/social icons (none needed).

## Imagery

All Unsplash, hotlinkable CDN:

- Hero: `photo-1566073771259-6a8506099945` — operator at console
- Final CTA: `photo-1455587734955-081b22074882` — hotel at sunset

## Why this matters

The Grand Palm rebrand protects the public guest surface from looking like a B2B pitch. But operators — the actual buyers — need a sales-grade view of what the platform does. `/platform` is that view: 21 modules, 4 suites, the ecosystem diagram, the comparison, and the pricing. It is wired to deep links into the staff console so a curious operator can click "Open module" and see the actual surface (PMS, Front Desk, F&B, etc.) in context, completing the operator's mental model of the platform.

## What's not here yet

- Real comparison competitor names (kept generic for legal safety — we don't want to misrepresent)
- Live "Request demo" form — currently deep-links to the auth page with `signup=1`, which is fine for the sandbox demo but should become a proper form when the platform has a real sales inbox
- Per-module deep pages — modules still point to the existing staff console routes, not module marketing pages
