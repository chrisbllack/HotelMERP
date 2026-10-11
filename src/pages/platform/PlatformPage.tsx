// INFINEON Enterprise Platform — operator-facing marketing site.
// Route /platform · Hotel management software, 13 modules, 3 suite groups,
// pricing, partner CTA. Inspired by cloudhotelerp / djubo / yanolja conventions.

import { useState } from "react";
import { Link } from "react-router-dom";
import {
  ArrowRight, ArrowUpRight, BarChart3, BedDouble, Building2, Calculator, CalendarCheck2,
  Check, CheckCircle2, ChevronRight, ClipboardList, Coffee, ConciergeBell, Cpu, Database,
  FileText, Globe, Headphones, KeyRound, Layers, LineChart, Lock, Mail, MessageSquare,
  Percent, Phone, Plug, ShieldCheck, Sparkles, Store, Tag, Users, Wallet, Waves, Wrench,
} from "lucide-react";

const HERO_IMAGE =
  "https://images.unsplash.com/photo-1566073771259-6a8506099945?ixlib=rb-4.1.0&auto=format&fit=crop&w=2400&q=80";

type Suite = "operations" | "distribution" | "intelligence" | "guest";
type Module = {
  id: string;
  name: string;
  suite: Suite;
  tagline: string;
  href: string;
  icon: typeof BedDouble;
};

const MODULES: Module[] = [
  // Operations
  { id: "pms",       suite: "operations", name: "Property Management",   tagline: "Reservations · room rack · check-in / out",          icon: BedDouble,    href: "/rooms" },
  { id: "frontdesk", suite: "operations", name: "Front Desk Console",    tagline: "Live arrivals, departures & in-house queue",         icon: ConciergeBell, href: "/front-desk" },
  { id: "hk",        suite: "operations", name: "Housekeeping Board",    tagline: "Room status, attendant routing, minibar",             icon: ClipboardList, href: "/housekeeping" },
  { id: "maint",     suite: "operations", name: "Maintenance & PM",      tagline: "Work orders, preventive schedules, asset register",  icon: Wrench,       href: "/maintenance" },
  { id: "shift",     suite: "operations", name: "Shift Handover",        tagline: "Logbook, supervisor notes, follow-ups",               icon: FileText,     href: "/shift-handover" },
  { id: "audit",     suite: "operations", name: "Night Audit",           tagline: "Trial balance, postings, exception queue",           icon: Calculator,   href: "/night-audit" },
  { id: "fnb",       suite: "operations", name: "POS & Restaurants",     tagline: "Tables, kitchen display, recipe costing",            icon: Coffee,       href: "/pos" },
  { id: "inv",       suite: "operations", name: "Inventory & Procurement", tagline: "Stores, GRN, P2P, supplier contracts, stocktake",  icon: Store,        href: "/inventory" },
  { id: "events",    suite: "operations", name: "Events & Banqueting",   tagline: "Function diary, BEOs, deposits",                      icon: CalendarCheck2, href: "/events" },
  { id: "hr",        suite: "operations", name: "HR & Payroll",          tagline: "Employees, attendance, payroll cycles",              icon: Users,        href: "/hr" },
  { id: "hw",        suite: "operations", name: "Hardware Integration",  tagline: "Printers, scanners, locks, sensors",                 icon: Cpu,          href: "/hardware" },
  // Distribution
  { id: "book",      suite: "distribution", name: "Booking Engine",        tagline: "Direct site, abandoned cart, upsells",                icon: Globe,        href: "/book" },
  { id: "chan",      suite: "distribution", name: "Channel Manager",       tagline: "Booking.com, Expedia, Airbnb, Agoda sync",            icon: Plug,         href: "/channels" },
  { id: "yield",     suite: "distribution", name: "Yield & Revenue",       tagline: "Dynamic pricing, restrictions, length-of-stay rules", icon: Percent,      href: "/yield-management" },
  { id: "res",       suite: "distribution", name: "Reservations",          tagline: "GDS, OTA, group bookings, deposits",                  icon: KeyRound,     href: "/reservations" },
  { id: "pay",       suite: "distribution", name: "Payments",              tagline: "Card, transfer, USSD, Pay-at-desk, chargebacks",     icon: Wallet,       href: "/payments" },
  // Intelligence
  { id: "fin",       suite: "intelligence", name: "Finance & USALI",        tagline: "GL, AP/AR, P&L by department, USALI statements",    icon: BarChart3,    href: "/finance" },
  { id: "crm",       suite: "intelligence", name: "CRM & Loyalty",          tagline: "Profiles, segments, repeat-guest history, RFM",     icon: Tag,          href: "/crm" },
  { id: "rep",       suite: "intelligence", name: "Reports & BI",           tagline: "20+ standard reports + custom builder",              icon: LineChart,    href: "/reports" },
  { id: "demand",    suite: "intelligence", name: "Demand & Forecasting",   tagline: "Pace, pick-up, segmentation, forward-looking P&L",   icon: Layers,      href: "/reports" },
  // Guest
  { id: "guest",     suite: "guest",       name: "Guest Portal",           tagline: "Pre-arrival, in-stay requests, mobile check-in",     icon: MessageSquare, href: "/auth" },
];

const SUITE_META: Record<Suite, { label: string; tagline: string; n: number; chip: string }> = {
  operations:    { label: "Operations",    tagline: "Run the property from a single console — front desk, housekeeping, maintenance, F&B, finance.", n: 11, chip: "11 modules" },
  distribution:  { label: "Distribution",  tagline: "Sell rooms everywhere — direct, OTAs, GDS, groups, channels — with one inventory of truth.",        n: 5,  chip: "5 modules" },
  intelligence:  { label: "Intelligence",  tagline: "See the property the way the owner sees it — USALI, segment P&L, demand forecasts.",                  n: 4,  chip: "4 modules" },
  guest:         { label: "Guest",         tagline: "Every step from reservation to check-out, on the guest's phone — without an app download.",        n: 1,  chip: "1 module" },
};

const SUITE_ORDER: Suite[] = ["operations", "distribution", "intelligence", "guest"];

const INTEGRATIONS = [
  "Booking.com", "Expedia", "Agoda", "Airbnb", "HotelBeds",
  "Stripe", "Paystack", "Flutterwave", "QuickBooks", "Sage",
  "Mews", "Cloudbeds", "SiteMinder", "Lightspeed", "Oracle OPERA",
];

const PRICING = [
  {
    tier: "Boutique",
    eyebrow: "Up to 30 keys",
    monthly: "₦ 350,000",
    yearly: "₦ 3,500,000",
    blurb: "For independent properties, B&Bs, and small boutique hotels. One property, all 21 modules, all themes.",
    modules: ["PMS + Front desk", "Booking engine", "Channel manager", "Housekeeping", "POS & inventory", "Reports & BI"],
    cta: "Request a 14-day trial",
    highlight: false,
  },
  {
    tier: "Grand",
    eyebrow: "30 – 200 keys",
    monthly: "₦ 1,250,000",
    yearly: "₦ 12,500,000",
    blurb: "For full-service hotels and small groups. Adds yield management, night audit, USALI, and SSO.",
    modules: ["Everything in Boutique", "Yield & revenue", "Night audit", "USALI statements", "HR & payroll", "Single sign-on (SAML)"],
    cta: "Request a 14-day trial",
    highlight: true,
  },
  {
    tier: "Estate",
    eyebrow: "Multi-property",
    monthly: "On request",
    yearly: "On request",
    blurb: "For chains, resorts, and ownership groups. Adds multi-property ledger, owner statements, dedicated CSM.",
    modules: ["Everything in Grand", "Multi-property ledger", "Owner statements", "Audit-grade roles", "Dedicated CSM", "99.95% SLA + status page"],
    cta: "Talk to sales",
    highlight: false,
  },
];

const COMPARISONS = [
  { row: "Modules in one platform",   a: "21",    b: "13",  c: "8",   d: "12" },
  { row: "Multi-property ledger",     a: "✓",     b: "✓",   c: "✗",   d: "✓" },
  { row: "USALI out of the box",      a: "✓",     b: "✗",   c: "✗",   d: "✗" },
  { row: "POS with recipe costing",   a: "✓",     b: "✓",   c: "✓",   d: "✗" },
  { row: "Hardware integration",      a: "Built-in", b: "Paid add-on", c: "Paid add-on", d: "Paid add-on" },
  { row: "On-prem option",            a: "✓",     b: "✗",   c: "✗",   d: "✗" },
  { row: "Pricing",                   a: "From ₦350k/mo", b: "From $499/mo", c: "From $199/mo", d: "From $600/mo" },
];

const TESTIMONIALS = [
  {
    quote: "We replaced four systems with INFINEON. Our GMs learn the whole thing in a week — and we don't have a six-figure integrator on retainer anymore.",
    name: "Dele Adebayo",
    role: "Owner-operator · 4 properties, Lagos & Abuja",
  },
  {
    quote: "The night audit used to take until lunch. It now closes itself by 02:30. I have my evenings back.",
    name: "Funmi Ogunyemi",
    role: "Group controller · West Africa hospitality group",
  },
  {
    quote: "The booking engine's abandoned-cart recovery added 11% to our direct revenue in the first quarter. We didn't even know we were losing that.",
    name: "Carlos Mendes",
    role: "Revenue manager · 92-key resort",
  },
];

const FAQ = [
  { q: "How long does implementation take?", a: "A standard 30-key property takes 10 working days from kickoff to go-live. We import your rate plan, room types, historical reservations, and guest profiles; we map your chart of accounts; we connect your channel manager and payment processor. Multi-property rollouts are phased and run from 6 weeks." },
  { q: "Do we have to be online to operate?", a: "No. The Front Desk, Housekeeping, and POS modules run fully offline. Bookings made offline reconcile automatically when the connection returns. The same goes for the booking engine — guests can book and pay, and the confirmation email is sent the moment the property is back online." },
  { q: "Is INFINEON USALI-compliant?", a: "Yes. The Finance module ships with the 11th edition Uniform System of Accounts for the Lodging Industry, and a side-by-side 10th/11th reconciliation. We also ship the Indian 7-categories chart of accounts and a UNECA-aligned model for the East African market." },
  { q: "Can we run on our own servers?", a: "Yes. The Estate tier ships as a self-hosted container, with a one-button installer for any Debian 12 host. We also offer a fully managed cloud option (the default for Boutique and Grand). Data residency is your choice." },
  { q: "What does onboarding cost?", a: "Nothing. The 14-day trial includes full onboarding, training for up to 10 staff, and a 30-day money-back guarantee. The first month is billed only after you've gone live. After that, our help-desk is included in the subscription — there are no per-ticket fees." },
];

export default function PlatformPage() {
  const [suite, setSuite] = useState<Suite | "all">("all");

  const visible = MODULES.filter((m) => suite === "all" || m.suite === suite);

  return (
    <div className="min-h-screen bg-brand-950 text-brand-50">
      <a href="#modules" className="skip-link">Skip to modules</a>

      {/* Top nav */}
      <header className="absolute inset-x-0 top-0 z-30">
        <div className="mx-auto flex h-20 max-w-7xl items-center justify-between px-5 lg:px-10">
          <Link to="/" className="flex items-center gap-2.5">
            <div className="flex h-10 w-10 items-center justify-center rounded-md bg-gradient-to-br from-brand-500 to-brand-800 font-display text-lg text-gold-400 ring-1 ring-gold-500/40">
              I
            </div>
            <div className="leading-tight">
              <p className="font-display text-base text-brand-50">INFINEON</p>
              <p className="text-[10px] uppercase tracking-[0.25em] text-gold-300/80">Enterprise Platform</p>
            </div>
          </Link>
          <nav className="hidden items-center gap-7 text-sm text-brand-100/85 lg:flex">
            <a href="#modules" className="hover:text-gold-300">Modules</a>
            <a href="#ecosystem" className="hover:text-gold-300">How it fits together</a>
            <a href="#integrations" className="hover:text-gold-300">Integrations</a>
            <a href="#pricing" className="hover:text-gold-300">Pricing</a>
            <a href="#faq" className="hover:text-gold-300">FAQ</a>
          </nav>
          <div className="flex items-center gap-3">
            <Link
              to="/"
              className="hidden text-sm text-brand-200/70 transition-colors hover:text-gold-300 sm:inline"
            >
              See a live property →
            </Link>
            <Link
              to="/auth?returnTo=%2Fdashboard"
              className="rounded-md border border-white/20 px-3.5 py-2 text-sm font-medium text-brand-50 transition-colors hover:bg-white/10"
            >
              Sign in
            </Link>
            <Link
              to="/auth?returnTo=%2Fdashboard&signup=1"
              className="flex items-center gap-1.5 rounded-md bg-gradient-to-r from-gold-400 to-gold-500 px-4 py-2 text-sm font-semibold text-brand-950 transition-all hover:from-gold-300 hover:to-gold-400"
            >
              Request demo <ArrowRight size={14} />
            </Link>
          </div>
        </div>
      </header>

      <main id="main">
        {/* Hero */}
        <section className="relative">
          <div className="relative h-[88vh] min-h-[640px] w-full overflow-hidden">
            <img src={HERO_IMAGE} alt="Hotel operator reviewing a console" className="h-full w-full object-cover" />
            <div className="absolute inset-0 bg-gradient-to-b from-black/65 via-black/40 to-black" />
            <div className="absolute inset-0 bg-gradient-to-r from-black/80 via-black/30 to-transparent" />
          </div>
          <div className="absolute inset-0 flex flex-col justify-center">
            <div className="mx-auto w-full max-w-7xl px-5 lg:px-10">
              <span className="inline-flex items-center gap-2 rounded-full border border-gold-400/40 bg-black/40 px-3.5 py-1.5 text-[11px] font-medium uppercase tracking-[0.2em] text-gold-200 backdrop-blur">
                <Sparkles size={11} className="text-gold-300" /> Hotel management software · 21 modules
              </span>
              <h1 className="mt-6 max-w-4xl font-display text-5xl leading-[1.05] text-white sm:text-6xl lg:text-7xl">
                One stop for <br className="hidden sm:block" />
                <em className="not-italic bg-gradient-to-r from-gold-200 via-gold-300 to-gold-500 bg-clip-text text-transparent">
                  every hotel operation
                </em>
                .
              </h1>
              <p className="mt-6 max-w-2xl text-lg text-white/85">
                Reservations, front desk, housekeeping, F&B, inventory, finance, channel manager,
                yield, and guest experience — in one platform your GMs can learn in a week, run
                offline, and deploy on your own servers.
              </p>
              <div className="mt-8 flex flex-wrap items-center gap-3">
                <Link
                  to="/auth?returnTo=%2Fdashboard&signup=1"
                  className="group flex items-center gap-2 rounded-md bg-gradient-to-r from-gold-400 to-gold-500 px-6 py-3.5 text-sm font-semibold text-brand-950 shadow-xl transition-all hover:from-gold-300 hover:to-gold-400"
                >
                  Request a 14-day trial
                  <ArrowRight size={16} className="transition-transform group-hover:translate-x-0.5" />
                </Link>
                <Link
                  to="/book"
                  className="flex items-center gap-2 rounded-md border border-white/30 bg-white/5 px-6 py-3.5 text-sm font-medium text-white backdrop-blur transition-colors hover:bg-white/15"
                >
                  See a live property
                </Link>
              </div>
              <div className="mt-7 flex flex-wrap items-center gap-x-6 gap-y-2 text-xs text-white/70">
                <span className="flex items-center gap-1.5"><Check size={12} className="text-gold-300" /> 14-day trial, no card</span>
                <span className="flex items-center gap-1.5"><Check size={12} className="text-gold-300" /> Onboarding included</span>
                <span className="flex items-center gap-1.5"><Check size={12} className="text-gold-300" /> 30-day money-back</span>
                <span className="flex items-center gap-1.5"><Check size={12} className="text-gold-300" /> Self-host option</span>
              </div>
            </div>
          </div>
        </section>

        {/* Social proof bar */}
        <section className="border-y border-brand-800/60 bg-brand-900/40">
          <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-center gap-x-10 gap-y-3 px-5 py-6 text-xs text-brand-200/70 sm:px-10">
            <span className="flex items-center gap-2 font-medium text-brand-100">
              <span className="h-1.5 w-1.5 animate-pulse-dot rounded-full bg-emerald-400" /> 240+ properties · 19 countries
            </span>
            <span>·</span>
            <span>Used in Lagos, Mumbai, Manila, Mexico City, and Cairo</span>
            <span>·</span>
            <span>Member — Leading Hotels of the World</span>
          </div>
        </section>

        {/* Modules — the big selling point */}
        <section id="modules" className="bg-brand-950 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div className="max-w-2xl">
                <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">The product suite</p>
                <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">
                  Twenty-one integrated modules. Four suites. One platform.
                </h2>
                <p className="mt-4 text-sm text-brand-100/70 sm:text-base">
                  Every module talks to every other module. The room you sell in Booking.com is the
                  room the housekeeping attendant sees, the room the F&B waiter bills to, and the
                  room the night-audit posts revenue against.
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <SuiteChip active={suite === "all"}           label="All"            count={MODULES.length} onClick={() => setSuite("all")} />
                <SuiteChip active={suite === "operations"}    label="Operations"     count={SUITE_META.operations.n} onClick={() => setSuite("operations")} />
                <SuiteChip active={suite === "distribution"}  label="Distribution"   count={SUITE_META.distribution.n} onClick={() => setSuite("distribution")} />
                <SuiteChip active={suite === "intelligence"}  label="Intelligence"   count={SUITE_META.intelligence.n} onClick={() => setSuite("intelligence")} />
                <SuiteChip active={suite === "guest"}         label="Guest"          count={SUITE_META.guest.n} onClick={() => setSuite("guest")} />
              </div>
            </div>

            <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {visible.map((m) => {
                const Icon = m.icon;
                return (
                  <Link
                    key={m.id}
                    to={m.href}
                    className="group flex flex-col rounded-2xl border border-brand-800/70 bg-brand-900/40 p-5 transition-all hover:border-gold-500/40 hover:bg-brand-900/60"
                  >
                    <div className="flex items-start justify-between">
                      <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-gold-500/10 text-gold-400 ring-1 ring-gold-500/30">
                        <Icon size={18} />
                      </div>
                      <ArrowUpRight
                        size={15}
                        className="text-brand-200/40 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-gold-300"
                      />
                    </div>
                    <p className="mt-4 font-display text-lg text-brand-50">{m.name}</p>
                    <p className="mt-1.5 text-sm text-brand-100/70">{m.tagline}</p>
                    <div className="mt-4 flex items-center justify-between border-t border-brand-800/60 pt-3 text-[11px]">
                      <span className="rounded-full border border-brand-700 bg-brand-950/60 px-2 py-0.5 uppercase tracking-wider text-brand-200/70">
                        {m.suite}
                      </span>
                      <span className="text-brand-200/60 group-hover:text-gold-300">Open module →</span>
                    </div>
                  </Link>
                );
              })}
            </div>

            {/* Suite narrative */}
            <div className="mt-14 grid gap-4 lg:grid-cols-4">
              {SUITE_ORDER.map((s) => (
                <div
                  key={s}
                  className="rounded-2xl border border-brand-800/70 bg-gradient-to-br from-brand-900/60 to-brand-950/40 p-6"
                >
                  <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">
                    {SUITE_META[s].chip}
                  </p>
                  <p className="mt-2 font-display text-xl text-brand-50">{SUITE_META[s].label}</p>
                  <p className="mt-2 text-sm text-brand-100/70">{SUITE_META[s].tagline}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* Ecosystem diagram */}
        <section id="ecosystem" className="border-t border-brand-800/60 bg-brand-900/30 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="max-w-2xl">
              <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">How it fits together</p>
              <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">
                One data model. Twenty-one modules. Zero double entry.
              </h2>
            </div>

            <div className="mt-10 grid gap-4 lg:grid-cols-12">
              <DiagramNode col="lg:col-span-3" label="Channels"            hint="Booking.com · Expedia · Agoda · GDS"          icon={Globe}        tier="in"  />
              <DiagramNode col="lg:col-span-2" label="Booking engine"       hint="Direct site"                                  icon={Globe}        tier="in"  />
              <DiagramNode col="lg:col-span-3" label="PMS"                  hint="Room rack · reservations · check-in"          icon={BedDouble}    tier="core" />
              <DiagramNode col="lg:col-span-2" label="Yield"                hint="Dynamic pricing"                              icon={Percent}      tier="in"  />
              <DiagramNode col="lg:col-span-2" label="Payments"             hint="Card · transfer · USSD"                        icon={Wallet}       tier="in"  />
              <DiagramNode col="lg:col-span-3" label="Front desk"           hint="Live arrivals & departures"                   icon={ConciergeBell} tier="out" />
              <DiagramNode col="lg:col-span-3" label="Housekeeping"         hint="Room status · attendants"                     icon={ClipboardList} tier="out" />
              <DiagramNode col="lg:col-span-3" label="Maintenance"          hint="Work orders · PM"                             icon={Wrench}       tier="out" />
              <DiagramNode col="lg:col-span-3" label="POS & Restaurants"    hint="Tables · KDS · recipe costing"                icon={Coffee}       tier="out" />
              <DiagramNode col="lg:col-span-4" label="Inventory & Procurement" hint="Stores · GRN · P2P · stocktake"             icon={Store}        tier="out" />
              <DiagramNode col="lg:col-span-4" label="HR & Payroll"         hint="Employees · attendance"                       icon={Users}        tier="out" />
              <DiagramNode col="lg:col-span-4" label="Hardware"             hint="Printers · locks · sensors"                   icon={Cpu}          tier="out" />
              <DiagramNode col="lg:col-span-3" label="Night audit"          hint="Closes by 02:30"                              icon={Calculator}   tier="audit" />
              <DiagramNode col="lg:col-span-3" label="Finance & USALI"      hint="AP/AR · P&L · statements"                     icon={BarChart3}    tier="audit" />
              <DiagramNode col="lg:col-span-3" label="Reports & BI"         hint="20+ standard + custom"                        icon={LineChart}    tier="audit" />
              <DiagramNode col="lg:col-span-3" label="CRM & Loyalty"        hint="Profiles · segments · RFM"                    icon={Tag}          tier="audit" />
              <DiagramNode col="lg:col-span-12" label="Guest portal — pre-arrival, in-stay requests, mobile check-in" hint="Branded for your property. No app download." icon={MessageSquare} tier="guest" wide />
            </div>
          </div>
        </section>

        {/* Integrations */}
        <section id="integrations" className="bg-brand-950 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="grid gap-10 lg:grid-cols-2 lg:items-center">
              <div>
                <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Integrations</p>
                <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">Plays well with what you already use.</h2>
                <p className="mt-4 text-sm leading-relaxed text-brand-100/80 sm:text-base">
                  We ship native integrations for the OTAs, payment processors, accounting systems,
                  and hardware vendors that hospitality teams already trust. Most of them are turned
                  on with a key and a checkbox — no integrator required.
                </p>
                <div className="mt-6 flex flex-wrap gap-2">
                  {INTEGRATIONS.map((i) => (
                    <span
                      key={i}
                      className="rounded-full border border-brand-800 bg-brand-900/40 px-3 py-1.5 text-[11px] text-brand-100/85"
                    >
                      {i}
                    </span>
                  ))}
                </div>
                <Link
                  to="/auth?returnTo=%2Fdashboard&signup=1"
                  className="mt-7 inline-flex items-center gap-1.5 text-sm font-medium text-gold-300 hover:text-gold-200"
                >
                  Don't see what you use? Ask us. <ChevronRight size={14} />
                </Link>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <Capability icon={Lock} title="SAML SSO" detail="Okta, Azure AD, Google Workspace — out of the box." />
                <Capability icon={Database} title="Open data" detail="Postgres logical replication, Parquet exports, S3 sync." />
                <Capability icon={Plug} title="Webhooks" detail="Every event, typed, signed, and retryable." />
                <Capability icon={ShieldCheck} title="PCI-aware" detail="Card data is tokenised; we never see PANs." />
                <Capability icon={Headphones} title="API & SDK" detail="REST + OpenAPI 3.1 · TypeScript & Python SDKs." />
                <Capability icon={Waves} title="Webhooks & queues" detail="Retried, ordered, with a dead-letter inspector." />
              </div>
            </div>
          </div>
        </section>

        {/* Comparison */}
        <section className="border-t border-brand-800/60 bg-brand-900/30 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="max-w-2xl">
              <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Comparison</p>
              <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">Where we differ.</h2>
            </div>
            <div className="mt-10 overflow-hidden rounded-2xl border border-brand-800/70 bg-brand-950/40">
              <table className="w-full text-left text-sm">
                <thead className="bg-brand-900/60 text-[11px] uppercase tracking-wider text-brand-200/70">
                  <tr>
                    <th className="px-5 py-3 font-medium">Capability</th>
                    <th className="px-5 py-3 font-medium text-gold-300">INFINEON</th>
                    <th className="px-5 py-3 font-medium">Cloud competitor A</th>
                    <th className="px-5 py-3 font-medium">Cloud competitor B</th>
                    <th className="px-5 py-3 font-medium">Cloud competitor C</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-brand-800/50">
                  {COMPARISONS.map((row) => (
                    <tr key={row.row} className="hover:bg-brand-900/30">
                      <td className="px-5 py-3 text-brand-100/90">{row.row}</td>
                      <td className="px-5 py-3 font-medium text-gold-300">{row.a}</td>
                      <td className="px-5 py-3 text-brand-100/70">{row.b}</td>
                      <td className="px-5 py-3 text-brand-100/70">{row.c}</td>
                      <td className="px-5 py-3 text-brand-100/70">{row.d}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <p className="border-t border-brand-800/70 bg-brand-900/30 px-5 py-3 text-[11px] text-brand-200/55">
                Comparison based on publicly published pricing and feature pages, October 2026.
              </p>
            </div>
          </div>
        </section>

        {/* Testimonials */}
        <section className="bg-brand-950 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="max-w-2xl">
              <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">What operators say</p>
              <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">From the operators' room.</h2>
            </div>
            <div className="mt-10 grid gap-4 md:grid-cols-3">
              {TESTIMONIALS.map((t) => (
                <figure key={t.name} className="rounded-2xl border border-brand-800/70 bg-brand-900/40 p-6">
                  <CheckCircle2 className="text-gold-400/80" size={20} />
                  <blockquote className="mt-4 font-display text-lg leading-snug text-brand-50 sm:text-xl">
                    "{t.quote}"
                  </blockquote>
                  <figcaption className="mt-5 border-t border-brand-800/70 pt-4 text-xs text-brand-200/70">
                    <p className="font-medium text-brand-100">{t.name}</p>
                    <p>{t.role}</p>
                  </figcaption>
                </figure>
              ))}
            </div>
          </div>
        </section>

        {/* Pricing */}
        <section id="pricing" className="border-t border-brand-800/60 bg-brand-900/30 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="grid items-end gap-6 lg:grid-cols-[1fr_auto]">
              <div className="max-w-2xl">
                <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Pricing</p>
                <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">Simple, transparent, per property.</h2>
                <p className="mt-3 text-sm text-brand-100/75">
                  One platform, 21 modules, three plans. Billed monthly or annually. No per-module
                  fees, no per-booking commission, no per-staff licensing.
                </p>
              </div>
              <BillingToggle />
            </div>

            <div className="mt-10 grid gap-4 lg:grid-cols-3">
              {PRICING.map((p) => (
                <article
                  key={p.tier}
                  className={
                    "flex h-full flex-col rounded-2xl border p-6 shadow-panel " +
                    (p.highlight
                      ? "border-gold-500/60 bg-gradient-to-br from-gold-500/10 via-brand-900/60 to-brand-950/40"
                      : "border-brand-800/70 bg-brand-900/40")
                  }
                >
                  <div className="flex items-center justify-between">
                    <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">
                      {p.eyebrow}
                    </p>
                    {p.highlight ? (
                      <span className="rounded-full bg-gradient-to-r from-gold-400 to-gold-500 px-2 py-0.5 text-[10px] font-semibold text-brand-950">
                        Most popular
                      </span>
                    ) : null}
                  </div>
                  <p className="mt-2 font-display text-2xl text-brand-50">{p.tier}</p>
                  <p className="mt-1.5 text-sm text-brand-100/70">{p.blurb}</p>
                  <p className="mt-5 font-display text-3xl text-brand-50">{p.monthly}</p>
                  <p className="text-[11px] text-brand-200/55">per property / month · billed monthly</p>
                  <p className="mt-1 text-[11px] text-brand-200/55">or {p.yearly} per year — save two months</p>
                  <ul className="mt-5 space-y-1.5 text-sm text-brand-100/85">
                    {p.modules.map((m) => (
                      <li key={m} className="flex items-start gap-2">
                        <Check size={14} className="mt-0.5 flex-none text-gold-400" /> {m}
                      </li>
                    ))}
                  </ul>
                  <div className="flex-1" />
                  <Link
                    to="/auth?returnTo=%2Fdashboard&signup=1"
                    className={
                      "mt-6 flex items-center justify-center gap-1.5 rounded-lg px-4 py-2.5 text-sm font-semibold transition-all " +
                      (p.highlight
                        ? "bg-gradient-to-r from-gold-400 to-gold-500 text-brand-950 hover:from-gold-300 hover:to-gold-400"
                        : "border border-brand-700 bg-brand-950/50 text-brand-50 hover:border-gold-500/40")
                    }
                  >
                    {p.cta} <ArrowRight size={14} />
                  </Link>
                </article>
              ))}
            </div>
            <p className="mt-6 text-center text-[11px] text-brand-200/55">
              All prices in Nigerian Naira. International billing available on request.
              <br />Setup, onboarding, and training are included with every plan.
            </p>
          </div>
        </section>

        {/* FAQ */}
        <section id="faq" className="bg-brand-950 py-20">
          <div className="mx-auto max-w-4xl px-5 lg:px-10">
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">FAQ</p>
            <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">Common questions, before the call.</h2>
            <div className="mt-8 divide-y divide-brand-800/60 rounded-2xl border border-brand-800/70 bg-brand-900/40">
              {FAQ.map((f, idx) => (
                <FaqItem key={f.q} q={f.q} a={f.a} openDefault={idx === 0} />
              ))}
            </div>
          </div>
        </section>

        {/* Final CTA */}
        <section className="relative overflow-hidden border-t border-brand-800/60">
          <div className="absolute inset-0">
            <img
              src="https://images.unsplash.com/photo-1455587734955-081b22074882?ixlib=rb-4.1.0&auto=format&fit=crop&w=2400&q=80"
              alt="Hotel operator at sunset"
              className="h-full w-full object-cover"
            />
            <div className="absolute inset-0 bg-gradient-to-b from-brand-950/80 via-brand-950/70 to-brand-950" />
          </div>
          <div className="relative mx-auto max-w-3xl px-5 py-24 text-center">
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Request a trial</p>
            <h2 className="mt-4 font-display text-3xl text-brand-50 sm:text-4xl">
              Run the property in 14 days. Decide in 30.
            </h2>
            <p className="mt-4 text-sm text-brand-100/80 sm:text-base">
              We set up your property, train your team, and stay with you for the first month. If
              it's not the right fit, we refund the subscription — no friction, no exit interview.
            </p>
            <div className="mt-7 flex flex-wrap items-center justify-center gap-3">
              <Link
                to="/auth?returnTo=%2Fdashboard&signup=1"
                className="inline-flex items-center gap-2 rounded-md bg-gradient-to-r from-gold-400 to-gold-500 px-6 py-3.5 text-sm font-semibold text-brand-950 shadow-xl transition-all hover:from-gold-300 hover:to-gold-400"
              >
                Request a 14-day trial <ArrowRight size={16} />
              </Link>
              <a
                href="mailto:sales@infineon.example"
                className="inline-flex items-center gap-2 rounded-md border border-white/30 bg-white/5 px-6 py-3.5 text-sm font-medium text-white backdrop-blur transition-colors hover:bg-white/15"
              >
                <Mail size={14} /> Talk to sales
              </a>
            </div>
            <div className="mt-6 flex flex-wrap items-center justify-center gap-x-6 gap-y-2 text-xs text-brand-200/65">
              <span className="flex items-center gap-1.5"><Phone size={12} className="text-gold-300" /> +234 1 700 INFINEON</span>
              <span className="flex items-center gap-1.5"><Building2 size={12} className="text-gold-300" /> 14 Palm Springs Avenue, Victoria Island, Lagos</span>
            </div>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="border-t border-brand-800/60 bg-brand-950 py-12">
        <div className="mx-auto grid max-w-7xl gap-10 px-5 sm:grid-cols-2 lg:grid-cols-4 lg:px-10">
          <div>
            <div className="flex items-center gap-2.5">
              <div className="flex h-9 w-9 items-center justify-center rounded-md bg-gradient-to-br from-brand-500 to-brand-800 font-display text-brand-950 text-gold-400">
                I
              </div>
              <div className="leading-tight">
                <p className="font-display text-sm text-brand-50">INFINEON</p>
                <p className="text-[10px] uppercase tracking-widest text-gold-300/80">Enterprise Platform</p>
              </div>
            </div>
            <p className="mt-3 text-xs leading-relaxed text-brand-200/65">
              Hotel management software built in Lagos since 2024, deployed in 19 countries.
              <br />A product of InfinityPlus Solutions & Services Ltd.
            </p>
          </div>
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Product</p>
            <ul className="mt-3 space-y-2 text-sm text-brand-100/80">
              <li><a href="#modules" className="hover:text-gold-300">All modules</a></li>
              <li><a href="#integrations" className="hover:text-gold-300">Integrations</a></li>
              <li><a href="#pricing" className="hover:text-gold-300">Pricing</a></li>
              <li><Link to="/" className="hover:text-gold-300">Live property demo</Link></li>
            </ul>
          </div>
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Resources</p>
            <ul className="mt-3 space-y-2 text-sm text-brand-100/80">
              <li><a href="#" className="hover:text-gold-300">Documentation</a></li>
              <li><a href="#" className="hover:text-gold-300">API reference</a></li>
              <li><a href="#" className="hover:text-gold-300">Status</a></li>
              <li><a href="#" className="hover:text-gold-300">Changelog</a></li>
            </ul>
          </div>
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Contact</p>
            <ul className="mt-3 space-y-2 text-sm text-brand-100/80">
              <li>+234 1 700 INFINEON</li>
              <li>sales@infineon.example</li>
              <li>14 Palm Springs Avenue, VI, Lagos</li>
            </ul>
          </div>
        </div>
        <p className="mx-auto mt-10 max-w-7xl px-5 text-[11px] text-brand-200/40 lg:px-10">
          © 2026 InfinityPlus Solutions & Services Ltd. All rights reserved.
        </p>
      </footer>
    </div>
  );
}

function SuiteChip({
  active, label, count, onClick,
}: { active: boolean; label: string; count: number; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        "flex items-center gap-1.5 rounded-full border px-3.5 py-1.5 text-xs transition-colors " +
        (active
          ? "border-gold-500/60 bg-gold-500/15 text-gold-200"
          : "border-brand-800 bg-brand-950/40 text-brand-100/75 hover:border-brand-700")
      }
    >
      <span className="h-1.5 w-1.5 rounded-full bg-gold-400" />
      {label}
      <span className="rounded-full bg-brand-950/60 px-1.5 text-[10px] text-brand-200/65">{count}</span>
    </button>
  );
}

function DiagramNode({
  col, label, hint, icon: Icon, tier, wide,
}: {
  col: string; label: string; hint: string;
  icon: typeof BedDouble; tier: "in" | "core" | "out" | "audit" | "guest"; wide?: boolean;
}) {
  const tone: Record<typeof tier, string> = {
    in:    "border-sky-400/40 bg-sky-500/10 text-sky-100",
    core:  "border-gold-500/60 bg-gradient-to-br from-gold-500/20 to-gold-700/10 text-gold-200",
    out:   "border-brand-700 bg-brand-900/60 text-brand-100",
    audit: "border-emerald-500/40 bg-emerald-500/10 text-emerald-100",
    guest: "border-rose-400/40 bg-rose-500/10 text-rose-100",
  };
  return (
    <div className={"rounded-2xl border p-4 " + tone[tier] + " " + col}>
      <div className="flex items-center gap-2">
        <Icon size={15} />
        <p className={"font-display text-sm " + (wide ? "" : "leading-tight")}>{label}</p>
      </div>
      <p className="mt-1 text-[11px] opacity-75">{hint}</p>
    </div>
  );
}

function Capability({
  icon: Icon, title, detail,
}: { icon: typeof BedDouble; title: string; detail: string }) {
  return (
    <div className="rounded-xl border border-brand-800/70 bg-brand-950/60 p-4">
      <div className="flex h-8 w-8 items-center justify-center rounded-md bg-gold-500/10 text-gold-400 ring-1 ring-gold-500/30">
        <Icon size={15} />
      </div>
      <p className="mt-3 font-medium text-brand-50">{title}</p>
      <p className="mt-0.5 text-xs text-brand-100/65">{detail}</p>
    </div>
  );
}

function FaqItem({ q, a, openDefault }: { q: string; a: string; openDefault?: boolean }) {
  const [open, setOpen] = useState(!!openDefault);
  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between gap-4 px-5 py-4 text-left"
        aria-expanded={open}
      >
        <span className="font-display text-base text-brand-50 sm:text-lg">{q}</span>
        <ChevronRight
          size={16}
          className={"flex-none text-brand-200/60 transition-transform " + (open ? "rotate-90 text-gold-300" : "")}
        />
      </button>
      {open ? (
        <div className="px-5 pb-5 text-sm leading-relaxed text-brand-100/80">{a}</div>
      ) : null}
    </div>
  );
}

function BillingToggle() {
  const [annual, setAnnual] = useState(false);
  return (
    <div className="inline-flex items-center rounded-full border border-brand-800 bg-brand-900/50 p-1 text-sm">
      <button
        onClick={() => setAnnual(false)}
        className={
          "rounded-full px-4 py-1.5 text-xs font-semibold transition-colors " +
          (!annual ? "bg-gradient-to-r from-gold-400 to-gold-500 text-brand-950" : "text-brand-100/75 hover:text-brand-50")
        }
      >
        Monthly
      </button>
      <button
        onClick={() => setAnnual(true)}
        className={
          "flex items-center gap-1.5 rounded-full px-4 py-1.5 text-xs font-semibold transition-colors " +
          (annual ? "bg-gradient-to-r from-gold-400 to-gold-500 text-brand-950" : "text-brand-100/75 hover:text-brand-50")
        }
      >
        Annual
        <span className={"rounded-full px-1.5 py-0.5 text-[10px] " + (annual ? "bg-brand-950/30 text-brand-950" : "bg-emerald-500/15 text-emerald-200")}>
          save 2 mo
        </span>
      </button>
    </div>
  );
}
