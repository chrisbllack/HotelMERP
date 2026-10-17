# INFINEON Enterprise Platform

**Project TITAN** — Commercial Enterprise Platform  
**Company:** InfinityPlus Solutions & Services Ltd.

## Overview

INFINEON is a modern, enterprise-grade hospitality platform built on-premises first, with optional cloud and hybrid deployment. The first vertical is **Hospitality** — centered on the Lodging & Guest Operations module — targeting 50–150 room hotels.

## Quick Start

The supported first-build deployment model is a native host install. The current
repository contains the Go backend packages and the Vite frontend, but the
release entrypoint referenced by the existing Go/Docker artifacts
(`cmd/infineon`) is not present in this checkout yet. Use the frontend workflow
for UI development until that backend entrypoint is restored.

```bash
# Install the repository's current frontend dependencies
npm install

# Start local development (binds Vite to port 3000)
npm run dev
```

Access the development UI at `http://127.0.0.1:3000`.

For the intended native Go + PostgreSQL installation, follow
[Native Deployment](docs/DEPLOYMENT.md). That runbook documents the supported
Linux and Windows host matrix, local-only binding, service managers, health
checks, and optional Cloudflare Tunnel exposure. Existing Docker artifacts are
retained for compatibility but are not the default deployment path.

## Architecture

- **Backend:** Go 1.22+ (single binary, embedded UI)
- **Frontend:** React + TypeScript + Vite (embedded in Go binary)
- **Mobile:** Flutter (Android + iOS)
- **Database:** PostgreSQL 16
- **Deployment:** On-Premises First, Cloud Optional, Hybrid Ready

## Product Architecture: Modules

The platform is organized as **13 main modules** carrying **62 submodules**
([ADR-0006](docs/ADR/0006-ModuleHierarchy.md), [Module Catalog](docs/modules/MODULES.md)).
**Lodging & Guest Operations is the center of gravity** — the product is a hotel
system that includes finance, F&B, and CRM, not a bundle of suites.

| Code | Main Module | Highlights |
|------|-------------|-----------|
| M-LOD | Lodging & Guest Operations ● | PMS: rooms, rates, reservations, front desk, housekeeping, maintenance, night audit |
| M-FNB | Food & Beverage ● | Multi-outlet POS, KOT/KDS, menu engineering, bar control, minibar |
| M-EVT | Events & Conference ◐ | Function diary, BEO contracts, catering, AV resources |
| M-CRM | Guest Experience & CRM ● | Guest 360, loyalty, campaigns, feedback & reputation |
| M-DST | Distribution & Commerce ◐ | Channel manager, booking engine, corporate & groups |
| M-INV | Supply Chain ● | Multi-store inventory, procure-to-pay, recipe costing, assets |
| M-FIN | Finance & Accounting ● | GL, AR/AP, cash & banking, fixed assets, tax & compliance |
| M-PRC | Payments ● | Gateway & terminal adapters, mobile money, settlement & reconciliation |
| M-HR | Workforce ● | HR core, time & attendance, payroll, demand-based scheduling |
| M-BUS | Intelligence & AI ◐ | Dashboards, 100+ reports, forecasting, AI copilot, revenue optimization |
| M-HDW | Device Integration ● | Vendor-neutral locks, IPTV, PBX, printers, KDS via Driver SDK |
| M-MOB | Mobile & Guest Apps ◐ | Staff apps (HK/engineering/manager/waiter), guest app, kiosks & owner portals |
| M-PLT | Platform Core ● | Identity & RBAC, org & licensing, workflow, notifications, audit, sync |

● Core · ◐ Differentiator · ○ Premium tier. Full submodule & feature depth:
[docs/modules/](docs/modules/MODULES.md).

Legacy suite names (IHES, IRES, IFES, IIES, ICES, IMES, IAES, IBIP, IHF) are
retired as architecture and preserved only as licensing bundles — see the
crosswalk in the [Module Catalog](docs/modules/MODULES.md#legacy-product-family-crosswalk).

## Documentation

- [Module Catalog & Architecture](docs/modules/MODULES.md)
- [Module Specs](docs/modules/) (M-LOD, M-FNB, M-EVT, M-CRM, M-DST, M-INV, M-FIN, M-PRC, M-HR, M-BUS, M-HDW, M-MOB, M-PLT, M-SEC)
- [API](docs/API/)
- [Offline Mode](docs/OfflineMode.md)
- [ADR](docs/ADR/) — incl. [ADR-0006 Module Hierarchy](docs/ADR/0006-ModuleHierarchy.md)
- **Enterprise Layer** — [Editions & Licensing](docs/Enterprise/EDITIONS.md),
  [Integration Contracts](docs/Enterprise/INTEGRATION-CONTRACTS.md),
  [Multi-Property](docs/Enterprise/MULTI-PROPERTY.md),
  [Compliance](docs/Enterprise/COMPLIANCE.md),
  [Operations](docs/Enterprise/OPERATIONS.md)

## License

Proprietary — InfinityPlus Solutions & Services Ltd.
