import { Check, Palette } from "lucide-react";
import { THEMES, useTheme, type ThemeId } from "../store/theme";
import { cn } from "../lib/utils";

interface ThemeSwitcherProps {
  /** Show as a compact horizontal strip (toolbar) or expanded grid (settings page). */
  variant?: "grid" | "strip";
  className?: string;
}

/**
 * Lets the operator preview and switch the active theme. The grid variant is
 * intended for the System Admin page; the strip variant fits the AppShell
 * topbar so a power user can flip themes without leaving their current view.
 */
export function ThemeSwitcher({ variant = "grid", className }: ThemeSwitcherProps) {
  const { theme, setTheme } = useTheme();

  if (variant === "strip") {
    return (
      <div className={cn("flex items-center gap-1.5", className)} role="radiogroup" aria-label="Theme">
        {THEMES.map((t) => {
          const active = t.id === theme;
          return (
            <button
              key={t.id}
              type="button"
              role="radio"
              aria-checked={active}
              aria-label={t.name}
              title={t.name}
              onClick={() => setTheme(t.id)}
              data-theme={t.id}
              className={cn(
                "h-6 w-6 rounded-full border-2 transition-all",
                active
                  ? "scale-110 border-gold-400 shadow-[0_0_0_2px_var(--color-gold-500)]"
                  : "border-brand-800 hover:scale-105"
              )}
              style={{ background: t.swatch.primary }}
            />
          );
        })}
      </div>
    );
  }

  return (
    <div className={cn("space-y-3", className)}>
      <div className="flex items-center gap-2 text-sm font-semibold text-brand-50">
        <Palette size={16} className="text-gold-400" />
        <span>Theme</span>
        <span className="text-xs font-normal text-brand-200/60">— choose how the console feels</span>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
        {THEMES.map((t) => (
          <ThemeCard
            key={t.id}
            id={t.id}
            active={t.id === theme}
            onSelect={() => setTheme(t.id)}
          />
        ))}
      </div>
    </div>
  );
}

function ThemeCard({
  id,
  active,
  onSelect,
}: {
  id: ThemeId;
  active: boolean;
  onSelect: () => void;
}) {
  const meta = THEMES.find((t) => t.id === id)!;
  return (
    <button
      type="button"
      onClick={onSelect}
      data-theme={id}
      className={cn(
        "group relative overflow-hidden rounded-xl border text-left transition-all",
        active
          ? "border-gold-400 shadow-[0_0_0_2px_var(--color-gold-500)]"
          : "border-brand-800 hover:border-brand-600"
      )}
      style={{ background: meta.swatch.surface }}
    >
      {/* Mini preview chrome — swatches, faux header, faux card */}
      <div className="relative h-28 w-full overflow-hidden">
        <div
          className="absolute inset-0"
          style={{
            background: `linear-gradient(135deg, ${meta.swatch.surface} 0%, ${meta.swatch.primary} 100%)`,
          }}
        />
        <div
          className="absolute left-3 top-3 h-2 w-12 rounded-full"
          style={{ background: meta.swatch.accent }}
        />
        <div
          className="absolute left-3 top-7 h-1.5 w-20 rounded-full opacity-60"
          style={{ background: meta.swatch.accent }}
        />
        <div
          className="absolute bottom-3 left-3 right-3 rounded-md p-2"
          style={{
            background: `${meta.swatch.primary}cc`,
            borderTop: `2px solid ${meta.swatch.accent}`,
          }}
        >
          <div className="h-1 w-16 rounded" style={{ background: meta.swatch.accent }} />
          <div className="mt-1 h-1 w-24 rounded opacity-50" style={{ background: meta.swatch.accent }} />
        </div>
        {active ? (
          <span className="absolute right-2 top-2 flex h-6 w-6 items-center justify-center rounded-full bg-gold-500 text-brand-950">
            <Check size={14} strokeWidth={3} />
          </span>
        ) : null}
      </div>

      <div className="p-3">
        <div className="flex items-center justify-between">
          <p className="font-display text-sm font-semibold" style={{ color: meta.swatch.accent }}>
            {meta.name}
          </p>
          <div className="flex gap-1">
            <span
              className="h-3 w-3 rounded-full border border-white/20"
              style={{ background: meta.swatch.primary }}
              aria-hidden="true"
            />
            <span
              className="h-3 w-3 rounded-full border border-white/20"
              style={{ background: meta.swatch.accent }}
              aria-hidden="true"
            />
            <span
              className="h-3 w-3 rounded-full border border-white/20"
              style={{ background: meta.swatch.surface }}
              aria-hidden="true"
            />
          </div>
        </div>
        <p className="mt-0.5 text-[11px] uppercase tracking-wider opacity-60" style={{ color: meta.swatch.accent }}>
          {meta.inspiredBy}
        </p>
        <p className="mt-1 text-xs leading-relaxed" style={{ color: meta.swatch.accent, opacity: 0.85 }}>
          {meta.tagline}
        </p>
      </div>
    </button>
  );
}
