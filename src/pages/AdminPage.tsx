import { Card, CardHeader, Badge, Table, Td, StatCard } from "../components/ui/primitives";
import { PageHeader } from "../components/ui/primitives";
import { auditLog, systemSettings, users } from "../data/demo";
import { ROLE_LABELS } from "../types";
import { Settings, ShieldCheck, UserCog, Palette } from "lucide-react";
import { ThemeSwitcher } from "../components/ThemeSwitcher";
import { useTheme } from "../store/theme";

const ROLE_SCOPE: Record<string, string> = {
  admin: "All modules · user & system management · licensing",
  general_manager: "All modules read · approvals · rate override",
  front_desk: "Reservations · check-in/out · folios · walk-ins",
  cashier: "Folios · payments · shift close",
  housekeeping: "Room board · task lifecycle",
  fnb: "POS · KDS · menu availability",
  accountant: "GL · journals · settlement · AR",
  hr: "Employees · departments · payroll",
};

const LICENSING = [
  ["Licensed to", "InfinityPlus Solutions & Services Ltd."],
  ["Property", "Grand Palm Hotel Lagos (GPHL)"],
  ["Edition", "Enterprise — all 13 modules"],
  ["License key", "IFN-ENT-••••-••••-7F2A (hardware-bound)"],
  ["Offline grace", "14 days without license server contact"],
  ["Support tier", "24/7 platinum · 1h response"],
];

export default function AdminPage() {
  const { meta } = useTheme();
  return (
    <div className="space-y-6">
      <PageHeader
        title="System Administration"
        subtitle="M-PLT · users & roles, property settings, licensing and the platform audit surface"
        actions={
          <span className="flex items-center gap-2 rounded-full border border-brand-800/70 bg-brand-900/60 px-3 py-1.5 text-xs text-brand-200">
            <Palette size={12} className="text-gold-400" />
            Active theme: <span className="font-medium text-gold-300">{meta.name}</span>
          </span>
        }
      />

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard label="Users" value={String(users.length)} sub="active demo accounts" />
        <StatCard label="Roles" value={String(Object.keys(ROLE_LABELS).length)} sub="permission sets" accent="gold" />
        <StatCard label="Settings" value={String(systemSettings.length)} sub="property & system scope" />
        <StatCard label="Audit events" value={String(auditLog.length)} sub="last 24 hours" accent="sky" />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader title="Users & Roles" subtitle="Role-scoped access — the sidebar adapts to each role" />
          <Table head={["User", "Role", "Scope", "Status"]}>
            {users.map((u) => (
              <tr key={u.id} className="hover:bg-brand-800/30">
                <Td>
                  <span className="flex items-center gap-2">
                    <UserCog size={14} className="text-brand-300" />
                    <span>
                      <span className="block text-sm font-medium text-brand-50">{u.name}</span>
                      <span className="block text-[11px] text-brand-200/50">{u.email}</span>
                    </span>
                  </span>
                </Td>
                <Td><span className="text-xs text-gold-300">{ROLE_LABELS[u.role]}</span></Td>
                <Td className="max-w-[220px] text-xs text-brand-200/60">{ROLE_SCOPE[u.role]}</Td>
                <Td>
                  <Badge className="border-emerald-400/40 bg-emerald-500/15 text-emerald-200">{u.status}</Badge>
                </Td>
              </tr>
            ))}
          </Table>
        </Card>

        <div className="space-y-4">
          <Card>
            <CardHeader title="Property Settings" subtitle="Key system configuration (read-only in demo)" />
            <ul className="divide-y divide-brand-800/50">
              {systemSettings.map((s) => (
                <li key={s.key} className="flex items-center justify-between gap-3 px-5 py-2.5">
                  <div className="min-w-0">
                    <p className="font-mono text-xs text-brand-50">{s.key}</p>
                    <p className="text-[10px] uppercase tracking-wider text-brand-200/40">{s.scope} scope</p>
                  </div>
                  <span className="flex items-center gap-1.5 text-sm text-gold-300">
                    <Settings size={12} /> {s.value}
                  </span>
                </li>
              ))}
            </ul>
          </Card>

          <Card>
            <CardHeader title="License" subtitle="Offline-first enterprise licensing" />
            <ul className="divide-y divide-brand-800/50">
              {LICENSING.map(([k, v]) => (
                <li key={k} className="flex items-center justify-between gap-3 px-5 py-2.5">
                  <span className="text-xs text-brand-200/60">{k}</span>
                  <span className="text-xs font-medium text-brand-50">{v}</span>
                </li>
              ))}
            </ul>
            <p className="flex items-center gap-2 border-t border-brand-800/70 px-5 py-3 text-xs text-brand-200/50">
              <ShieldCheck size={13} /> License validates locally on boot; grace countdown is visible in the admin header.
            </p>
          </Card>
        </div>
      </div>

      <Card>
        <CardHeader
          title="Visual Theme"
          subtitle="Pick a palette that matches the property's positioning — changes apply everywhere instantly"
        />
        <div className="p-5">
          <ThemeSwitcher />
        </div>
      </Card>
    </div>
  );
}
