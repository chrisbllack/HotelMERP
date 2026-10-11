import { Check } from "lucide-react";
import { Link } from "react-router-dom";

export type BookingStep = "dates" | "room" | "guests" | "confirm";

const STEPS: Array<{ id: BookingStep; n: number; label: string; to: (ctx: StepContext) => string }> = [
  { id: "dates", n: 1, label: "Choose dates", to: (c) => `/book${c ? `?${c}` : ""}` },
  { id: "room", n: 2, label: "Choose room", to: (_c) => `/book` },
  { id: "guests", n: 3, label: "Guest details", to: (_c) => `/book/guests` },
  { id: "confirm", n: 4, label: "Confirm", to: (_c) => `/booking/lookup` },
];

export type StepContext = string | undefined;

function isStepComplete(stepId: BookingStep, current: BookingStep): boolean {
  const order: BookingStep[] = ["dates", "room", "guests", "confirm"];
  return order.indexOf(stepId) < order.indexOf(current);
}

export function BookingSteps({
  current,
  context,
}: {
  current: BookingStep;
  context?: StepContext;
}) {
  return (
    <nav aria-label="Booking progress" className="mx-auto max-w-5xl px-5 pt-6">
      <ol className="flex flex-wrap items-center gap-2 text-xs">
        {STEPS.map((s, idx) => {
          const active = s.id === current;
          const done = isStepComplete(s.id, current);
          return (
            <li key={s.id} className="flex items-center gap-2">
              <Link
                to={s.to(context)}
                aria-current={active ? "step" : undefined}
                className={
                  active
                    ? "flex items-center gap-2 rounded-full bg-gold-500/15 px-3 py-1.5 ring-1 ring-gold-500/50"
                    : done
                      ? "flex items-center gap-2 rounded-full bg-brand-800/60 px-3 py-1.5 text-brand-100/80 hover:text-brand-50"
                      : "flex items-center gap-2 rounded-full px-3 py-1.5 text-brand-200/60"
                }
              >
                <span
                  className={
                    active
                      ? "flex h-5 w-5 items-center justify-center rounded-full bg-gold-500 font-semibold text-brand-950"
                      : done
                        ? "flex h-5 w-5 items-center justify-center rounded-full bg-brand-700 text-brand-50"
                        : "flex h-5 w-5 items-center justify-center rounded-full border border-brand-700 text-brand-200/70"
                  }
                >
                  {done ? <Check size={11} /> : s.n}
                </span>
                <span className="font-medium">{s.label}</span>
              </Link>
              {idx < STEPS.length - 1 ? (
                <span aria-hidden="true" className="hidden h-px w-6 bg-brand-700/70 sm:block" />
              ) : null}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
