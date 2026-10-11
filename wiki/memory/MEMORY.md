# Project Memory

One durable fact per bullet — decisions, preferences, constraints, gotchas.
Newest at the bottom. Curate in place: update what changed, delete what is
wrong, merge duplicates, flag contradictions with ⚠️. Use ⛔ for hard
"never do this" rules. When this file passes ~200 lines, consolidate detail
into topic files in this folder and keep a one-line pointer here.
Detail pages: `<topic>.md` next to this file. Journal: `log.md` (append-only).
Never store secrets, API keys, or tokens here.

## Facts
- Task: Run this app. — Outcome: The app is now running successfully on http://localhost:3000. The INFINEON Enterprise Platform hospitality management homepage loads with full navigation, hero section, and CTAs. **What I fixed to get it running:** 1. Created stub files for the missing Convex generated API (`src/convex/_generated/api.ts`, `dataModel.ts`, `server.ts`) — these were missing because no live Convex backend is provisioned in this sandbox. The stubs make imports resolve while `BOOKING_ENABLED` (gated on `VITE_CONVEX_URL`) stays false, so booking mutations gracefully no-op. 2. Installed the missing `react-is` dependency that Recharts requires. 3. Added `react-is`, `recharts`, and the Convex packages to Vite's `opti… #run-summary (fullstack-builder, 2026-10-11) <!-- productos-run:qteps7lj57y2cfbivwv84b5t -->
- Task: Attached is my Hotel ERP project Git repo. I want you to extend this project while bringing it in line with the top global industry best. I want to build an all-encompassing hospital/hotel PMS ERP with in-depth inventory, store and stock-k… — Outcome: I'll start by exploring the existing codebase to understand the architecture, then propose a comprehensive plan for transforming this into a world-class hospital/hotel PMS ERP with 7 switchable UI themes. Now I'll build the 7-theme system. First, let me update the CSS to define all 7 theme palettes. Excellent team structure — I'll capture this as durable project context, then dispatch the work in parallel across these role lanes. Now I'll dispatch parallel agents across the remaining lanes to complete the work. While the three lanes run in parallel, let me set up the project wiki to make the team structure durable. Now I have a clear picture. Let me start making the modifications. First, I'… #run-summary (fullstack-builder, 2026-10-11) <!-- productos-run:vn3j2oqvial4o7vh2xq00kci -->
