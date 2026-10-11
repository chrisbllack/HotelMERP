// M-LOD booking website — guest details + payment step (route /book/guests).

import { useMemo, useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useMutation, useAction } from "convex/react";
import { AlertCircle, BedDouble, CalendarDays, Loader2, Lock, Mail, Phone, ShieldCheck, User } from "lucide-react";
import { api } from "../../convex/_generated/api";
import { BookingShell } from "./BookingShell";
import { BookingSteps } from "./BookingSteps";
import { BOOKING_ENABLED, useEnsureSeeded } from "../../lib/booking-client";

const COUNTRIES = ["Nigeria", "Ghana", "Kenya", "South Africa", "United Kingdom", "United States", "Other"];

const ROOM_IMAGE: Record<string, string> = {
  STD: "https://images.unsplash.com/photo-1771775529138-a7a20ba7e032?ixlib=rb-4.1.0&auto=format&fit=crop&w=800&q=80",
  DLX: "https://images.unsplash.com/photo-1771775529138-a7a20ba7e032?ixlib=rb-4.1.0&auto=format&fit=crop&w=800&q=80",
  EXE: "https://images.unsplash.com/photo-1777169794972-12095816073b?ixlib=rb-4.1.0&auto=format&fit=crop&w=800&q=80",
  PRM: "https://images.unsplash.com/photo-1788927796219-35f6c27e6422?ixlib=rb-4.1.0&auto=format&fit=crop&w=800&q=80",
};

export default function GuestDetailsPage() {
  const [params] = useSearchParams();
  const navigate = useNavigate();

  const checkIn = params.get("checkIn") ?? "";
  const checkOut = params.get("checkOut") ?? "";
  const adults = Number(params.get("adults") ?? 2);
  const roomType = params.get("roomType") ?? "";
  const nights = useMemo(() => nightsBetween(checkIn, checkOut), [checkIn, checkOut]);

  const [form, setForm] = useState({
    firstName: "", lastName: "", email: "", phone: "", country: "Nigeria", specialRequests: "",
  });
  const [phase, setPhase] = useState<"form" | "creating" | "paying">("form");
  const [error, setError] = useState<string | null>(null);

  const createBooking = useMutation(api.booking.createBooking);
  const initializePayment = useAction(api.payments.initializePayment);
  useEnsureSeeded(BOOKING_ENABLED);

  const set = (k: keyof typeof form) => (e: { target: { value: string } }) =>
    setForm((f) => ({ ...f, [k]: e.target.value }));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setPhase("creating");
    try {
      const booking = await createBooking({
        checkIn,
        checkOut,
        roomTypeCode: roomType,
        adults,
        children: 0,
        firstName: form.firstName,
        lastName: form.lastName,
        email: form.email,
        phone: form.phone,
        country: form.country || undefined,
        specialRequests: form.specialRequests || undefined,
      });
      setPhase("paying");
      const pay = await initializePayment({
        code: booking.code,
        email: form.email,
        amountKobo: booking.depositKobo,
      });
      if (pay.mode === "paystack" && pay.authorizationUrl) {
        window.location.href = pay.authorizationUrl;
        return;
      }
      // Demo mode (pay at front desk) — go straight to confirmation.
      navigate(`/booking/confirmation?code=${encodeURIComponent(booking.code)}`, { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong — please try again.");
      setPhase("form");
    }
  };

  if (!checkIn || !checkOut || !roomType || nights < 1) {
    return (
      <BookingShell>
        <div className="mx-auto max-w-lg px-5 py-24 text-center">
          <AlertCircle className="mx-auto text-amber-300" size={28} />
          <h1 className="mt-4 font-display text-2xl">Missing booking details</h1>
          <p className="mt-2 text-sm text-brand-200/60">Start again by choosing your dates and room.</p>
          <Link
            to="/book"
            className="mt-6 inline-block rounded-lg bg-gradient-to-r from-gold-400 to-gold-500 px-5 py-2.5 text-sm font-semibold text-brand-950 hover:from-gold-300 hover:to-gold-400"
          >
            Back to search
          </Link>
        </div>
      </BookingShell>
    );
  }

  const busy = phase !== "form";

  return (
    <BookingShell>
      <BookingSteps
        current="guests"
        context={`checkIn=${checkIn}&checkOut=${checkOut}&adults=${adults}&roomType=${roomType}`}
      />

      <section className="mx-auto grid max-w-5xl gap-8 px-5 py-10 lg:grid-cols-[1fr_360px]">
        {/* Form */}
        <div>
          <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Step 3 of 4</p>
          <h1 className="mt-2 font-display text-3xl text-brand-50 sm:text-4xl">Tell us about you.</h1>
          <p className="mt-2 text-sm text-brand-100/70">
            We'll use this for your booking confirmation and a pre-arrival note from the concierge.
          </p>

          {error ? (
            <div className="mt-5 rounded-lg border border-rose-500/40 bg-rose-500/10 px-4 py-3 text-sm text-rose-200">
              {error}
            </div>
          ) : null}

          <form onSubmit={submit} className="mt-6 grid gap-4 sm:grid-cols-2">
            <Field icon={<User size={13} />} label="First name *" value={form.firstName} onChange={set("firstName")} />
            <Field icon={<User size={13} />} label="Last name *" value={form.lastName} onChange={set("lastName")} />
            <Field icon={<Mail size={13} />} label="Email *" type="email" value={form.email} onChange={set("email")} />
            <Field icon={<Phone size={13} />} label="Phone *" value={form.phone} onChange={set("phone")} placeholder="+234…" />

            <label className="block sm:col-span-2">
              <span className="mb-1 block text-xs text-brand-200/70">Country of residence</span>
              <select
                value={form.country}
                onChange={set("country")}
                className="w-full rounded-lg border border-brand-700 bg-brand-950/70 px-3 py-2.5 text-sm text-brand-50 outline-none focus:border-gold-500/60"
              >
                {COUNTRIES.map((c) => <option key={c} className="bg-brand-950 text-brand-50">{c}</option>)}
              </select>
            </label>

            <label className="block sm:col-span-2">
              <span className="mb-1 block text-xs text-brand-200/70">Special requests</span>
              <textarea
                rows={3}
                value={form.specialRequests}
                onChange={set("specialRequests")}
                placeholder="Airport pickup, high floor, dietary needs, late arrival…"
                className="w-full resize-none rounded-lg border border-brand-700 bg-brand-950/70 px-3 py-2.5 text-sm text-brand-50 outline-none focus:border-gold-500/60"
              />
              <span className="mt-1 block text-[10px] text-brand-200/45">
                We'll do our best to honour these — final confirmation arrives by email.
              </span>
            </label>

            <button
              type="submit"
              disabled={busy}
              className="flex items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-gold-400 to-gold-500 px-6 py-3.5 text-sm font-semibold text-brand-950 transition hover:from-gold-300 hover:to-gold-400 disabled:opacity-60 sm:col-span-2"
            >
              {phase === "creating" ? (
                <><Loader2 size={16} className="animate-spin" /> Creating your booking…</>
              ) : phase === "paying" ? (
                <><Loader2 size={16} className="animate-spin" /> Opening secure payment…</>
              ) : (
                <><Lock size={15} /> Confirm &amp; pay deposit</>
              )}
            </button>
            <p className="flex items-center justify-center gap-1.5 text-center text-[11px] text-brand-200/55 sm:col-span-2">
              <ShieldCheck size={13} className="text-gold-400" />
              Deposit charged securely via Paystack. Balance settles at the front desk on arrival.
            </p>
          </form>
        </div>

        {/* Summary rail */}
        <aside className="h-fit overflow-hidden rounded-2xl border border-brand-800/70 bg-brand-900/50 shadow-panel lg:sticky lg:top-24">
          <div className="relative h-32 overflow-hidden">
            <img
              src={ROOM_IMAGE[roomType] ?? ROOM_IMAGE.STD}
              alt={roomTypeLabel(roomType)}
              className="h-full w-full object-cover"
            />
            <div className="absolute inset-0 bg-gradient-to-t from-black/70 via-black/20 to-transparent" />
            <div className="absolute bottom-0 left-0 p-4">
              <p className="text-[10px] uppercase tracking-[0.2em] text-gold-300">Your room</p>
              <p className="font-display text-lg text-white">{roomTypeLabel(roomType)}</p>
            </div>
          </div>
          <div className="p-5">
            <dl className="space-y-3 text-sm">
              <Row icon={<CalendarDays size={13} />} k="Check-in" v={fmtDate(checkIn)} />
              <Row icon={<CalendarDays size={13} />} k="Check-out" v={fmtDate(checkOut)} />
              <Row icon={<BedDouble size={13} />} k="Nights" v={String(nights)} />
              <Row icon={<User size={13} />} k="Guests" v={`${adults} adult${adults > 1 ? "s" : ""}`} />
            </dl>
            <div className="my-4 border-t border-brand-800/70" />
            <dl className="space-y-3 text-sm">
              <Row k="Deposit due now" v="20% at checkout" />
              <Row k="Balance" v="Due at the hotel" />
            </dl>
            <div className="mt-5 rounded-lg border border-gold-500/30 bg-gold-500/5 p-3 text-[11px] leading-relaxed text-brand-100/75">
              <p className="font-medium text-gold-200">Free cancellation</p>
              <p className="mt-0.5">Cancel up to 48 hours before arrival for a full refund of your deposit.</p>
            </div>
          </div>
        </aside>
      </section>
    </BookingShell>
  );
}

function Field({
  icon, label, value, onChange, type = "text", placeholder,
}: {
  icon: React.ReactNode; label: string; value: string;
  onChange: (e: { target: { value: string } }) => void;
  type?: string; placeholder?: string;
}) {
  return (
    <label className="block">
      <span className="mb-1 flex items-center gap-1.5 text-xs text-brand-200/70">
        <span className="text-gold-400">{icon}</span>
        {label}
      </span>
      <input
        required
        type={type}
        value={value}
        onChange={onChange}
        placeholder={placeholder}
        className="w-full rounded-lg border border-brand-700 bg-brand-950/70 px-3 py-2.5 text-sm text-brand-50 outline-none transition-colors focus:border-gold-500/60"
      />
    </label>
  );
}

function Row({ icon, k, v }: { icon?: React.ReactNode; k: string; v: string }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <dt className="flex items-center gap-1.5 text-brand-200/65">
        {icon ? <span className="text-gold-400">{icon}</span> : null}
        {k}
      </dt>
      <dd className="text-right font-medium text-brand-50">{v}</dd>
    </div>
  );
}

function roomTypeLabel(code: string): string {
  const map: Record<string, string> = {
    STD: "Standard Room", DLX: "Deluxe Room", EXE: "Executive Suite", PRM: "Presidential Suite",
  };
  return map[code] ?? code;
}

function fmtDate(iso: string): string {
  const t = Date.parse(`${iso}T00:00:00Z`);
  if (Number.isNaN(t)) return iso;
  return new Date(t).toLocaleDateString("en-GB", { weekday: "short", day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });
}

function nightsBetween(a: string, b: string): number {
  const ta = Date.parse(`${a}T00:00:00Z`);
  const tb = Date.parse(`${b}T00:00:00Z`);
  if (Number.isNaN(ta) || Number.isNaN(tb)) return 0;
  return Math.max(0, Math.round((tb - ta) / 86_400_000));
}
