import { Link, NavLink } from "react-router-dom";
import { type ReactNode } from "react";
import { ArrowRight, Building2, Phone, ShieldCheck } from "lucide-react";

const NAV_LINKS = [
  { to: "/book", label: "Book a Stay" },
  { to: "/booking/lookup", label: "Manage Booking" },
  { to: "/auth?returnTo=%2Fdashboard", label: "Staff Console" },
];

export function BookingShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col bg-brand-950 text-brand-50">
      <header className="sticky top-0 z-30 border-b border-brand-800/60 bg-brand-950/85 backdrop-blur">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
          <Link to="/" className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-md bg-gradient-to-br from-gold-400 to-gold-600 font-display text-lg text-brand-950 ring-1 ring-white/10">
              G
            </div>
            <div className="leading-tight">
              <p className="font-display text-sm text-brand-50">Grand Palm</p>
              <p className="text-[10px] uppercase tracking-widest text-gold-300/80">Hotel Lagos · Reservations</p>
            </div>
          </Link>
          <nav className="flex items-center gap-1 sm:gap-2">
            {NAV_LINKS.map((l) => (
              <NavLink
                key={l.to}
                to={l.to}
                className={({ isActive }) =>
                  `rounded-lg px-3 py-2 text-sm transition-colors ${
                    isActive && !l.to.includes("auth")
                      ? "bg-brand-800/60 text-brand-50"
                      : "text-brand-100/75 hover:bg-brand-900/60 hover:text-brand-50"
                  }`
                }
              >
                {l.label}
              </NavLink>
            ))}
            <Link
              to="/book"
              className="ml-1 hidden items-center gap-1.5 rounded-md bg-gradient-to-r from-gold-400 to-gold-500 px-3.5 py-2 text-xs font-semibold text-brand-950 transition-all hover:from-gold-300 hover:to-gold-400 sm:flex"
            >
              Reserve <ArrowRight size={12} />
            </Link>
          </nav>
        </div>
      </header>

      <main className="flex-1">{children}</main>

      <footer className="border-t border-brand-800/60 py-10">
        <div className="mx-auto grid max-w-6xl gap-8 px-5 sm:grid-cols-3">
          <div>
            <p className="flex items-center gap-2 font-display text-sm text-brand-50">
              <Building2 size={15} className="text-gold-400" /> Grand Palm Hotel Lagos
            </p>
            <p className="mt-2 text-xs leading-relaxed text-brand-200/65">
              1 Lake Drive, Off Awolowo Road, Ikoyi, Lagos, Nigeria.
              <br />An independently-owned hotel since 1998.
            </p>
          </div>
          <div>
            <p className="flex items-center gap-2 text-sm text-brand-100/85">
              <Phone size={14} className="text-gold-400" /> Reservations
            </p>
            <p className="mt-2 text-xs text-brand-200/65">+234 1 461 2900</p>
            <p className="text-xs text-brand-200/65">reservations@grandpalm.ng</p>
          </div>
          <div>
            <p className="flex items-center gap-2 text-sm text-brand-100/85">
              <ShieldCheck size={14} className="text-gold-400" /> Secure payments
            </p>
            <p className="mt-2 text-xs leading-relaxed text-brand-200/65">
              Card, transfer & USSD via our PCI-aware checkout. Pay-at-desk also available.
            </p>
          </div>
        </div>
        <p className="mx-auto mt-8 max-w-6xl px-5 text-[11px] text-brand-200/40">
          © 2026 Grand Palm Hotel Lagos · InfinityPlus Solutions & Services Ltd.
        </p>
      </footer>
    </div>
  );
}
