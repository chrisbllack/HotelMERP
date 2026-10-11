---
type: concept
title: Team structure
scope: project
---

The user runs INFINEON as a 10-lane organisation. Work is dispatched in
parallel via the Agent tool — each lane owns a domain and may run
concurrently with others. Escalation order is bottom-up; sign-off order is
top-down.

## Lanes

| Lane | Owner role | Scope |
| --- | --- | --- |
| ME | Managing Director & Chief Visionary | Vision, business priorities, budgets, product approval, final sign-off |
| AI CTO / Engineering Manager | Translates vision into architecture, work plans, engineering standards, coordinates engineering lanes, owns ADRs |
| AI Product Manager | Requirements, roadmap, user stories, acceptance criteria |
| AI UI/UX Designer | User journeys, wireframes, design system, theme palettes, motion |
| AI Frontend Engineer | Web apps, responsive interfaces, accessibility, theming, state |
| AI Backend Engineer | APIs, business logic, integrations, database, schema migrations |
| AI QA / Test Engineer | Automated tests, regression coverage, bug triage, smoke checks |
| AI DevOps / SRE | CI/CD, Docker, monitoring, recovery, environment parity |
| AI Security Engineer | Access controls, vulnerability scanning, secrets, CSP / headers |
| AI Technical Writer | Documentation, installation, user manuals, API reference |

## How to dispatch

When a new feature request lands, identify the owning lane. For cross-cutting
work, decompose into independent chunks and run them concurrently. Always
keep the AI CTO lane aware of architectural changes.

## Escalation pattern

1. Lane does work
2. Lane writes back into the wiki / memory
3. AI CTO reviews for cross-lane impact
4. MD (user) signs off on roadmap-level changes
