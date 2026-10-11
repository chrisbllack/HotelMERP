import { useState, type ReactNode } from "react";
import { NavLink, Outlet } from "react-router-dom";
import {
  Activity, ArrowLeftRight, BarChart3, Bell, Building2, CreditCard, Crown,
  FileText, Gauge, HandCoins, LayoutDashboard, LogOut, Menu, Moon, Package, Plug,
  ShieldCheck, Sparkles, TrendingUp, UtensilsCrossed, Users, Wrench, X,
} from "lucide-react";
import { useAuth } from "../auth/auth";
import { PROPERTY } from "../data/demo";
import { ROLE_LABELS } from "../types";
import { cn } from "../lib/utils";
import { ThemeSwitcher } from "./ThemeSwitcher";

interface NavItem {
  label: string;
  to: string;
  icon: ReactNode;
  roles?: string[];
}

interface NavModule {
  label: string;
  code: string;
  badge?: string;
  items: NavItem[];
}

const NAV: NavModule[] = [
  {
    label: "Intelligence", code: "M-BUS",
    items: [{ label: "Executive Dashboard", to: "/dashboard", icon: <LayoutDashboard size={15} /> }],
  },
  {
    label: "Lodging & Guest Ops", code: "M-LOD", badge: "Core",
    items: [
      { label: "Front Desk", to: "/front-desk", icon: <ConciergeBell /> },
      { label: "Shift Handover", to: "/shift-handover", icon: <ArrowLeftRight size={15} /> },
      { label: "Room Rack", to: "/rooms", icon: <Building2 size={15} /> },
      { label: "Reservations", to: "/reservations", icon: <FileText size={15} /> },
      { label: "Housekeeping", to: "/housekeeping", icon: <Sparkles size={15} /> },
      { label: "Maintenance & PM", to: "/maintenance", icon: <Wrench size={15} /> },
      { label: "Night Audit", to: "/night-audit", icon: <Moon size={15} /> },
    ],
  },
  {
    label: "Food & Beverage", code: "M-FNB", badge: "Core",
    items: [{ label: "POS Terminal", to: "/pos", icon: <UtensilsCrossed size={15} /> }],
  },
  {
    label: "Supply Chain", code: "M-INV", badge: "Core",
    items: [{ label: "Inventory & Procurement", to: "/inventory", icon: <Package size={15} /> }],
  },
  {
    label: "Finance", code: "M-FIN", badge: "Core",
    items: [{ label: "General Ledger", to: "/finance", icon: <HandCoins size={15} /> }],
  },
  {
    label: "Workforce", code: "M-HR", badge: "Core",
    items: [{ label: "Employees & Payroll", to: "/hr", icon: <Users size={15} /> }],
  },
  {
    label: "Guest Experience", code: "M-CRM", badge: "Core",
    items: [{ label: "Guests & Loyalty", to: "/crm", icon: <Crown size={15} /> }],
  },
  {
    label: "Distribution & Revenue", code: "M-DST",
    items: [
      { label: "Yield & Dynamic Pricing", to: "/yield-management", icon: <TrendingUp size={15} /> },
      { label: "Channel Manager", to: "/channels", icon: <Plug size={15} /> },
    ],
  },
  {
    label: "Payments", code: "M-PRC",
    items: [{ label: "Transactions & Settlement", to: "/payments", icon: <CreditCard size={15} /> }],
  },
  {
    label: "Events", code: "M-EVT",
    items: [{ label: "Function Diary", to: "/events", icon: <BarChart3 size={15} /> }],
  },
  {
    label: "Devices", code: "M-HDW",
    items: [{ label: "Device Health", to: "/hardware", icon: <Activity size={15} /> }],
  },
  {
    label: "Platform", code: "M-PLT", badge: "Core",
    items: [
      { label: "Reports & Audit", to: "/reports", icon: <Gauge size={15} /> },
      { label: "System Admin", to: "/admin", icon: <ShieldCheck size={15} /> },
    ],
  },
];

function ConciergeBell() {
  // Small inline SVG: concierge bell
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M4 16a8 8 0 0 1 16 0" />
      <path d="M2 16h20" />
      <path d="M12 8V6" />
      <path d="M10 6h4" />
    </svg>
  );
}

export function AppShell() {
  const { user, logout } = useAuth();
  const [open, setOpen] = useState(false);

  return (
    <div className="flex min-h-screen bg-brand-950">
      {/* Sidebar */}
      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-40 flex w-72 flex-col border-r border-brand-800/70 bg-brand-900/60 backdrop-blur transition-transform lg:static lg:translate-x-0",
          open ? "translate-x-0" : "-translate-x-full",
        )}
      >
        <div className="flex items-center justify-between border-b border-brand-800/70 px-5 py-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-brand-500 to-brand-800 font-display text-lg text-gold-400 ring-1 ring-gold-500/40">
              I
            </div>
            <div>
              <p className="font-display text-sm leading-tight text-brand-50">INFINEON</p>
              <p className="text-[10px] uppercase tracking-widest text-gold-500/80">Enterprise</p>
            </div>
          </div>
          <button className="text-brand-200/70 hover:text-brand-50 lg:hidden" onClick={() => setOpen(false)} aria-label="Close menu">
            <X size={18} />
          </button>
        </div>

        <nav className="flex-1 space-y-4 overflow-y-auto px-3 py-4">
          {NAV.map((mod) => (
            <div key={mod.code}>
              <div className="flex items-center justify-between px-2 pb-1">
                <p className="text-[10px] font-semibold uppercase tracking-[0.14em] text-brand-200/40">
                  {mod.code} · {mod.label}
                </p>
                {mod.badge ? (
                  <span className="rounded bg-gold-500/15 px-1 text-[9px] font-semibold uppercase text-gold-400">{mod.badge}</span>
                ) : null}
              </div>
              <ul className="space-y-0.5">
                {mod.items.map((item) => (
                  <li key={item.to}>
                    <NavLink
                      to={item.to}
                      onClick={() => setOpen(false)}
                      className={({ isActive }) =>
                        cn(
                          "flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm transition-colors",
                          isActive
                            ? "bg-brand-500/20 font-medium text-brand-50 ring-1 ring-brand-400/30"
                            : "text-brand-100/70 hover:bg-brand-800/40 hover:text-brand-50",
                        )
                      }
                    >
                      <span className="text-brand-300">{item.icon}</span>
                      {item.label}
                    </NavLink>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </nav>

        <div className="border-t border-brand-800/70 px-4 py-3">
          <p className="text-[10px] text-brand-200/40">
            13 main modules · 65 submodules
            <br />
            v0.3.0 · Enterprise Edition
          </p>
        </div>
      </aside>

      {open ? <div className="fixed inset-0 z-30 bg-black/60 lg:hidden" onClick={() => setOpen(false)} /> : null}

      {/* Main */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-20 flex h-14 items-center justify-between gap-4 border-b border-brand-800/70 bg-brand-950/85 px-4 backdrop-blur lg:px-8">
          <div className="flex items-center gap-3">
            <button className="text-brand-100 lg:hidden" onClick={() => setOpen(true)} aria-label="Open menu">
              <Menu size={20} />
            </button>
            <div className="flex items-center gap-2 rounded-lg border border-brand-800/80 bg-brand-900/50 px-3 py-1.5">
              <Building2 size={14} className="text-gold-400" />
              <div className="leading-tight">
                <p className="text-xs font-medium text-brand-50">{PROPERTY.name}</p>
                <p className="text-[10px] text-brand-200/50">{PROPERTY.code} · Lagos, NG · 50 rooms</p>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div
              className="hidden items-center gap-2 rounded-lg border border-brand-800/80 bg-brand-900/50 px-3 py-1.5 md:flex"
              title="Active theme"
            >
              <span className="text-[10px] uppercase tracking-wider text-brand-200/60">Theme</span>
              <ThemeSwitcher variant="strip" />
            </div>
            <button className="relative rounded-lg p-2 text-brand-200/70 hover:bg-brand-900/60 hover:text-brand-50" aria-label="Notifications">
              <Bell size={17} />
              <span className="absolute right-1.5 top-1.5 h-1.5 w-1.5 rounded-full bg-gold-400" />
            </button>
            <div className="flex items-center gap-2.5">
              <div className="text-right leading-tight">
                <p className="text-xs font-medium text-brand-50">{user?.name}</p>
                <p className="text-[10px] text-brand-200/60">{user ? ROLE_LABELS[user.role] : ""}</p>
              </div>
              <button
                onClick={logout}
                className="flex items-center gap-1.5 rounded-lg border border-brand-800/80 px-2.5 py-1.5 text-xs text-brand-200/80 transition-colors hover:border-brand-600 hover:text-brand-50"
              >
                <LogOut size={13} /> Sign out
              </button>
            </div>
          </div>
        </header>

        <main className="flex-1 px-4 py-6 lg:px-8">
          <div className="mx-auto max-w-[1400px] animate-fade-up">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
