// M-LOD booking website — integrated, Convex-backed hotel booking flow.
// Route /book · SearchBar (dates/guests) + live availability from backend.

import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useQuery } from "convex/react";
import {
  CalendarDays, Check, ChevronRight, Clock, Coffee, ConciergeBell, Loader2,
  Sparkles, Star, Users, UtensilsCrossed, Waves, Wifi, Wine,
} from "lucide-react";
import { api } from "../../convex/_generated/api";
import { BookingShell } from "./BookingShell";
import { BookingSteps } from "./BookingSteps";
import { BOOKING_ENABLED, fmtKobo, useEnsureSeeded } from "../../lib/booking-client";
import { todayISO } from "../../lib/utils";

const HERO_IMAGE =
  "https://images.unsplash.com/photo-1775866914966-5bd2194c464f?ixlib=rb-4.1.0&auto=format&fit=crop&w=2400&q=80";

const ROOM_IMAGE: Record<string, string> = {
  deluxe:
    "https://images.unsplash.com/photo-1771775529138-a7a20ba7e032?ixlib=rb-4.1.0&auto=format&fit=crop&w=1200&q=80",
  suite:
    "https://images.unsplash.com/photo-1777169794972-12095816073b?ixlib=rb-4.1.0&auto=format&fit=crop&w=1200&q=80",
  penthouse:
    "https://images.unsplash.com/photo-1788927796219-35f6c27e6422?ixlib=rb-4.1.0&auto=format&fit=crop&w=1200&q=80",
};

const GUEST_PERKS: Array<{ icon: typeof Wifi; title: string; detail: string }> = [
  { icon: Coffee, title: "Breakfast included", detail: "Full buffet at the Atrium, 6:30–10:30am" },
  { icon: Wifi, title: "1 Gbps Wi-Fi", detail: "Free throughout, no codes, no limits" },
  { icon: Sparkles, title: "Spa credit", detail: "₦15,000 per night toward Aurora Spa treatments" },
  { icon: ConciergeBell, title: "Late check-out", detail: "2pm on request, subject to availability" },
];

function SearchBar({
  checkIn, checkOut, adults, busy,
  onCheckIn, onCheckOut, onAdults, onSearch,
}: {
  checkIn: string; checkOut: string; adults: number; busy: boolean;
  onCheckIn: (v: string) => void; onCheckOut: (v: string) => void;
  onAdults: (v: number) => void; onSearch: () => void;
}) {
  return (
    <div className="rounded-2xl border border-white/10 bg-brand-900/85 p-3 shadow-2xl backdrop-blur">
      <div className="grid gap-2 sm:grid-cols-4">
        <label className="block rounded-lg bg-brand-950/40 px-3 py-2.5 transition-colors hover:bg-brand-950/60">
          <span className="mb-1 flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-brand-200/60">
            <CalendarDays size={11} /> Check-in
          </span>
          <input
            type="date"
            min={todayISO}
            value={checkIn}
            onChange={(e) => onCheckIn(e.target.value)}
            className="w-full bg-transparent text-sm text-brand-50 outline-none [color-scheme:dark]"
          />
        </label>
        <label className="block rounded-lg bg-brand-950/40 px-3 py-2.5 transition-colors hover:bg-brand-950/60">
          <span className="mb-1 flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-brand-200/60">
            <CalendarDays size={11} /> Check-out
          </span>
          <input
            type="date"
            min={checkIn}
            value={checkOut}
            onChange={(e) => onCheckOut(e.target.value)}
            className="w-full bg-transparent text-sm text-brand-50 outline-none [color-scheme:dark]"
          />
        </label>
        <label className="block rounded-lg bg-brand-950/40 px-3 py-2.5 transition-colors hover:bg-brand-950/60">
          <span className="mb-1 flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-brand-200/60">
            <Users size={11} /> Guests
          </span>
          <select
            value={adults}
            onChange={(e) => onAdults(Number(e.target.value))}
            className="w-full bg-transparent text-sm text-brand-50 outline-none"
          >
            {[1, 2, 3, 4].map((n) => (
              <option key={n} value={n} className="bg-brand-950 text-brand-50">{n} guest{n > 1 ? "s" : ""}</option>
            ))}
          </select>
        </label>
        <button
          onClick={onSearch}
          disabled={busy}
          className="flex items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-gold-400 to-gold-500 px-4 py-3 text-sm font-semibold text-brand-950 transition hover:from-gold-300 hover:to-gold-400 disabled:opacity-50"
        >
          {busy ? <Loader2 size={15} className="animate-spin" /> : null} Check availability
        </button>
      </div>
    </div>
  );
}

export default function BookPage() {
  const [params] = useSearchParams();

  const [checkIn, setCheckIn] = useState(params.get("checkIn") ?? todayISO);
  const [checkOut, setCheckOut] = useState(params.get("checkOut") ?? addDays(params.get("checkIn") ?? todayISO, 2));
  const [adults, setAdults] = useState(Number(params.get("adults") ?? 2));
  const [submitted, setSubmitted] = useState({ checkIn, checkOut, adults });
  const [error, setError] = useState<string | null>(null);

  const { ready } = useEnsureSeeded(BOOKING_ENABLED);

  const availability = useQuery(
    api.booking.publicAvailability,
    BOOKING_ENABLED && ready && validRange(submitted.checkIn, submitted.checkOut)
      ? { checkIn: submitted.checkIn, checkOut: submitted.checkOut }
      : "skip",
  );

  const rows = useMemo(() => availability ?? [], [availability]);
  const nights = nightsBetween(submitted.checkIn, submitted.checkOut);

  const search = () => {
    setError(null);
    if (!validRange(checkIn, checkOut)) {
      setError("Check-out must be after check-in.");
      return;
    }
    setSubmitted({ checkIn, checkOut, adults });
  };

  return (
    <BookingShell>
      <a href="#results" className="skip-link">Skip to results</a>
      <BookingSteps
        current="dates"
        context={`checkIn=${submitted.checkIn}&checkOut=${submitted.checkOut}&adults=${submitted.adults}`}
      />

      {/* Editorial hero */}
      <section className="relative">
        <div className="relative h-[60vh] min-h-[440px] w-full overflow-hidden">
          <img
            src={HERO_IMAGE}
            alt="Grand Palm Hotel lobby at Grand Palm Lagos"
            className="h-full w-full object-cover"
          />
          <div className="absolute inset-0 bg-gradient-to-b from-black/45 via-black/20 to-black/85" />
          <div className="absolute inset-0 bg-gradient-to-r from-black/60 via-transparent to-black/40" />
        </div>
        <div className="absolute inset-0 flex flex-col justify-center">
          <div className="mx-auto w-full max-w-5xl px-5">
            <span className="inline-flex items-center gap-2 rounded-full border border-gold-400/40 bg-black/35 px-3 py-1 text-[10px] font-medium uppercase tracking-[0.2em] text-gold-200 backdrop-blur">
              <Star size={10} className="fill-gold-300 text-gold-300" /> Book direct · best rate
            </span>
            <h1 className="mt-5 max-w-3xl font-display text-4xl leading-[1.05] text-white sm:text-5xl lg:text-6xl">
              Reserve your stay at <em className="not-italic bg-gradient-to-r from-gold-200 via-gold-300 to-gold-500 bg-clip-text text-transparent">Grand Palm</em>.
            </h1>
            <p className="mt-4 max-w-xl text-base text-white/85 sm:text-lg">
              Live availability from the hotel's own inventory — the rooms you see here are exactly
              what the front desk sees.
            </p>
            <div id="search" className="mt-8 max-w-4xl">
              <SearchBar
                checkIn={checkIn} checkOut={checkOut} adults={adults}
                busy={availability === undefined && !!ready}
                onCheckIn={setCheckIn} onCheckOut={setCheckOut} onAdults={setAdults}
                onSearch={search}
              />
              {error ? <p className="mt-2 text-xs text-rose-300">{error}</p> : null}
            </div>
            <div className="mt-5 flex flex-wrap items-center gap-x-6 gap-y-1.5 text-xs text-white/70">
              <span className="flex items-center gap-1.5"><Check size={12} className="text-gold-300" /> Free cancellation up to 24h</span>
              <span className="flex items-center gap-1.5"><Check size={12} className="text-gold-300" /> No prepayment needed</span>
              <span className="flex items-center gap-1.5"><Check size={12} className="text-gold-300" /> 10% off vs. public rates</span>
            </div>
          </div>
        </div>
      </section>

      {/* Included with every stay */}
      <section className="border-b border-brand-800/60 bg-brand-900/30 py-12">
        <div className="mx-auto max-w-6xl px-5">
          <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Included with every room</p>
          <h2 className="mt-2 font-display text-2xl text-brand-50 sm:text-3xl">A few small things, included.</h2>
          <div className="mt-7 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {GUEST_PERKS.map((p) => (
              <div
                key={p.title}
                className="flex gap-3 rounded-xl border border-brand-800/70 bg-brand-950/50 p-4"
              >
                <div className="flex h-9 w-9 flex-none items-center justify-center rounded-lg bg-gold-500/10 text-gold-400 ring-1 ring-gold-500/30">
                  <p.icon size={16} />
                </div>
                <div>
                  <p className="text-sm font-medium text-brand-50">{p.title}</p>
                  <p className="mt-0.5 text-xs text-brand-100/65">{p.detail}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Availability results */}
      <section id="results" className="mx-auto max-w-6xl px-5 py-14">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Available rooms</p>
            <h2 className="mt-2 font-display text-2xl text-brand-50 sm:text-3xl">
              {nights} night{nights === 1 ? "" : "s"} · {submitted.adults} guest{submitted.adults > 1 ? "s" : ""}
            </h2>
            <p className="mt-1 text-sm text-brand-200/60">
              Live inventory — refreshes every minute.
            </p>
          </div>
          <div className="hidden items-center gap-1.5 rounded-full border border-brand-800 bg-brand-950/40 px-3 py-1.5 text-[11px] text-brand-200/70 sm:flex">
            <span className="h-1.5 w-1.5 animate-pulse-dot rounded-full bg-emerald-400" />
            Inventory live
          </div>
        </div>

        {!BOOKING_ENABLED ? (
          <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-200">
            Live booking is connecting to the property database… If this message persists, the Convex deployment URL is not configured.
          </div>
        ) : !ready || availability === undefined ? (
          <div className="grid gap-4">
            {[0, 1, 2].map((i) => (
              <div key={i} className="h-44 animate-pulse rounded-2xl border border-brand-800/70 bg-brand-900/40" />
            ))}
          </div>
        ) : rows.length === 0 ? (
          <div className="rounded-xl border border-brand-800/70 bg-brand-900/40 p-8 text-center text-sm text-brand-200/60">
            No room types configured for this property yet.
          </div>
        ) : (
          <div className="grid gap-4">
            {rows.map((r) => {
              const soldOut = r.available < 1;
              const fitsGuests = r.maxOccupancy >= submitted.adults;
              const img = ROOM_IMAGE[r.code] ?? ROOM_IMAGE.deluxe;
              return (
                <article
                  key={r.roomTypeId}
                  className="group grid overflow-hidden rounded-2xl border border-brand-800/70 bg-brand-900/40 shadow-panel transition-all hover:border-gold-500/40 sm:grid-cols-[280px_1fr_auto]"
                >
                  <div className="relative aspect-[4/3] overflow-hidden sm:aspect-auto">
                    <img
                      src={img}
                      alt={`${r.name} at Grand Palm`}
                      className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-105"
                    />
                    {r.available <= 2 && !soldOut ? (
                      <span className="absolute left-3 top-3 rounded-full border border-amber-300/40 bg-amber-500/30 px-2 py-0.5 text-[10px] font-medium text-amber-100 backdrop-blur">
                        Only {r.available} left
                      </span>
                    ) : null}
                  </div>
                  <div className="p-5">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="font-display text-lg text-brand-50">{r.name}</h3>
                      <span className="rounded-full border border-brand-700 bg-brand-950/60 px-2 py-0.5 text-[10px] uppercase tracking-wider text-brand-200/70">
                        {r.bedType} · sleeps {r.maxOccupancy}
                      </span>
                    </div>
                    <p className="mt-2 max-w-2xl text-sm text-brand-100/75">{r.description}</p>
                    <div className="mt-3 flex flex-wrap gap-1.5">
                      {r.amenities.slice(0, 6).map((a) => (
                        <span
                          key={a}
                          className="rounded-full bg-brand-800/50 px-2 py-0.5 text-[11px] text-brand-100/75"
                        >
                          {a}
                        </span>
                      ))}
                    </div>
                    <div className="mt-3 flex items-center gap-3 text-[11px] text-brand-200/55">
                      <span className="flex items-center gap-1"><Clock size={11} /> Check-in 14:00</span>
                      <span className="flex items-center gap-1"><Coffee size={11} /> Breakfast incl.</span>
                      <span className="flex items-center gap-1"><Wifi size={11} /> Free 1Gbps</span>
                    </div>
                  </div>
                  <div className="flex flex-col items-stretch justify-between gap-3 border-t border-brand-800/60 bg-brand-950/30 p-5 sm:items-end sm:border-l sm:border-t-0 sm:pl-6 sm:pr-6 sm:pt-5">
                    <div className="text-left sm:text-right">
                      <p className="text-[10px] uppercase tracking-wider text-brand-200/55">From</p>
                      <p className="font-display text-2xl text-gold-400">{fmtKobo(r.rateKobo)}</p>
                      <p className="text-[10px] text-brand-200/55">
                        per night
                        {nights > 1 && !soldOut ? ` · ${fmtKobo(r.rateKobo * nights, true)} total` : ""}
                      </p>
                    </div>
                    {soldOut ? (
                      <span className="rounded-lg border border-rose-500/40 bg-rose-500/10 px-4 py-2.5 text-sm font-medium text-rose-200">
                        Sold out
                      </span>
                    ) : !fitsGuests ? (
                      <span className="rounded-lg border border-brand-800 bg-brand-950/60 px-4 py-2.5 text-xs text-brand-200/55">
                        Sleeps {r.maxOccupancy} — too small for {submitted.adults} guests
                      </span>
                    ) : (
                      <Link
                        to={`/book/guests?checkIn=${submitted.checkIn}&checkOut=${submitted.checkOut}&adults=${submitted.adults}&roomType=${r.code}`}
                        className="flex items-center justify-center gap-1.5 rounded-lg bg-gradient-to-r from-gold-400 to-gold-500 px-5 py-2.5 text-sm font-semibold text-brand-950 transition hover:from-gold-300 hover:to-gold-400"
                      >
                        Reserve <ChevronRight size={15} />
                      </Link>
                    )}
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </section>

      {/* Editorial band */}
      <section className="border-t border-brand-800/60 bg-brand-900/30 py-14">
        <div className="mx-auto grid max-w-6xl gap-10 px-5 lg:grid-cols-3">
          <div className="lg:col-span-1">
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Why book direct</p>
            <h2 className="mt-2 font-display text-2xl text-brand-50 sm:text-3xl">Five reasons our guests return.</h2>
          </div>
          <ul className="space-y-4 text-sm text-brand-100/80 lg:col-span-2">
            <li className="flex gap-3">
              <span className="mt-0.5 flex h-6 w-6 flex-none items-center justify-center rounded-full bg-gold-500/15 text-[10px] font-semibold text-gold-300">01</span>
              <p><span className="font-medium text-brand-50">Best rate on our site.</span> We price-match any lower public rate within 24 hours.</p>
            </li>
            <li className="flex gap-3">
              <span className="mt-0.5 flex h-6 w-6 flex-none items-center justify-center rounded-full bg-gold-500/15 text-[10px] font-semibold text-gold-300">02</span>
              <p><span className="font-medium text-brand-50">Flexible cancellation.</span> Cancel up to 24 hours before check-in at no charge.</p>
            </li>
            <li className="flex gap-3">
              <span className="mt-0.5 flex h-6 w-6 flex-none items-center justify-center rounded-full bg-gold-500/15 text-[10px] font-semibold text-gold-300">03</span>
              <p><span className="font-medium text-brand-50">Hold ten per cent.</span> We always keep ten per cent of every room for direct bookers.</p>
            </li>
            <li className="flex gap-3">
              <span className="mt-0.5 flex h-6 w-6 flex-none items-center justify-center rounded-full bg-gold-500/15 text-[10px] font-semibold text-gold-300">04</span>
              <p><span className="font-medium text-brand-50">Welcome amenity.</span> A small plate, a hand-written note, and a complimentary upgrade when one is available.</p>
            </li>
            <li className="flex gap-3">
              <span className="mt-0.5 flex h-6 w-6 flex-none items-center justify-center rounded-full bg-gold-500/15 text-[10px] font-semibold text-gold-300">05</span>
              <p><span className="font-medium text-brand-50">Pay-at-desk available.</span> Hold the room with a card; pay when you arrive. No prepayment required.</p>
            </li>
          </ul>
        </div>
      </section>
    </BookingShell>
  );
}

// ── date utils (local) ───────────────────────────────────────────────────
function addDays(iso: string, days: number): string {
  const t = Date.parse(`${iso}T00:00:00Z`);
  if (Number.isNaN(t)) return iso;
  return new Date(t + days * 86_400_000).toISOString().slice(0, 10);
}

function nightsBetween(a: string, b: string): number {
  const ta = Date.parse(`${a}T00:00:00Z`);
  const tb = Date.parse(`${b}T00:00:00Z`);
  if (Number.isNaN(ta) || Number.isNaN(tb)) return 0;
  return Math.max(0, Math.round((tb - ta) / 86_400_000));
}

function validRange(a: string, b: string): boolean {
  return nightsBetween(a, b) > 0;
}
