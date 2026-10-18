import { useState, type FormEvent } from "react";
import { Link, Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { ArrowRight, KeyRound, LogIn, ShieldCheck, Sparkles, Star } from "lucide-react";
import { useAuth, DEMO_PASSWORD } from "../auth/auth";
import { users } from "../data/demo";
import { ROLE_LABELS } from "../types";

const DEMO_ACCOUNTS = users.slice(0, 5);

const SIDE_IMAGE =
  "https://images.unsplash.com/photo-1542314831-068cd1dbfeeb?ixlib=rb-4.1.0&auto=format&fit=crop&w=1600&q=80";

const QUOTES = [
  {
    quote: "The concierge remembered my daughter's name by day two. That's the difference.",
    author: "A weekend guest",
  },
  {
    quote: "Twelve years I've used their PMS demo. It feels like a real product, not a slide deck.",
    author: "An operator in training",
  },
  {
    quote: "Most property systems take a year. This one we were operating in a fortnight.",
    author: "A general manager",
  },
];

export default function AuthPage() {
  const { login, user } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const returnTo = params.get("returnTo") || "/dashboard";

  const [email, setEmail] = useState("admin@infineon.com");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const quote = QUOTES[Math.floor(Math.random() * QUOTES.length)];

  const destination = decodeURIComponent(returnTo);

  if (user) {
    return <Navigate to={destination} replace />;
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    // Tiny visual delay so the loading state reads on the button.
    await new Promise((r) => setTimeout(r, 250));
    const result = await login(email, password);
    setSubmitting(false);
    if (result.ok) {
      navigate(destination, { replace: true });
    } else {
      setError(result.error ?? "Sign-in failed.");
    }
  };

  const quickFill = (acctEmail: string) => {
    setEmail(acctEmail);
    setPassword(DEMO_PASSWORD);
    setError(null);
  };

  return (
    <div className="relative min-h-screen overflow-hidden bg-brand-950 lg:grid lg:grid-cols-[1fr_min(440px,40%)]">
      {/* Image side */}
      <aside className="relative hidden lg:block">
        <img
          src={SIDE_IMAGE}
          alt="Grand Palm Hotel Lagos — staff view of the lagoon"
          className="absolute inset-0 h-full w-full object-cover"
        />
        <div className="absolute inset-0 bg-gradient-to-b from-black/35 via-black/15 to-black/85" />
        <div className="absolute inset-0 bg-gradient-to-r from-black/40 to-transparent" />
        <div className="relative flex h-full flex-col justify-between p-12 text-white">
          <Link to="/" className="flex items-center gap-2.5">
            <div className="flex h-10 w-10 items-center justify-center rounded-md bg-gradient-to-br from-gold-400 to-gold-600 font-display text-lg text-brand-950 ring-1 ring-white/10">
              G
            </div>
            <div className="leading-tight">
              <p className="font-display text-base">Grand Palm</p>
              <p className="text-[10px] uppercase tracking-[0.25em] text-gold-300/80">Hotel Lagos · Enterprise</p>
            </div>
          </Link>

          <div className="max-w-md">
            <span className="inline-flex items-center gap-2 rounded-full border border-gold-400/40 bg-black/35 px-3 py-1 text-[10px] font-medium uppercase tracking-[0.2em] text-gold-200 backdrop-blur">
              <Star size={10} className="fill-gold-300 text-gold-300" /> For staff
            </span>
            <p className="mt-6 font-display text-3xl leading-snug">
              "{quote.quote}"
            </p>
            <p className="mt-4 text-xs text-white/60">— {quote.author}</p>
          </div>

          <div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-[11px] text-white/60">
            <span className="flex items-center gap-1.5"><ShieldCheck size={12} className="text-gold-300" /> Role-scoped sessions</span>
            <span className="flex items-center gap-1.5"><Sparkles size={12} className="text-gold-300" /> Audit trail on every action</span>
          </div>
        </div>
      </aside>

      {/* Form side */}
      <section className="relative flex min-h-screen items-center justify-center overflow-hidden px-5 py-12 lg:py-0">
        <div
          className="pointer-events-none absolute inset-0"
          aria-hidden="true"
        >
          <div className="absolute -top-32 left-1/2 h-[420px] w-[720px] -translate-x-1/2 rounded-full bg-brand-600/15 blur-3xl" />
          <div className="absolute bottom-0 right-[10%] h-56 w-56 rounded-full bg-gold-500/10 blur-3xl" />
        </div>

        <div className="relative w-full max-w-sm animate-fade-up">
          <div className="mb-7 lg:hidden">
            <Link to="/" className="flex items-center gap-2.5">
              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-gradient-to-br from-gold-400 to-gold-600 font-display text-lg text-brand-950">
                G
              </div>
              <div className="leading-tight">
                <p className="font-display text-base text-brand-50">Grand Palm</p>
                <p className="text-[10px] uppercase tracking-widest text-gold-300/80">Hotel Lagos</p>
              </div>
            </Link>
          </div>

          <h1 className="font-display text-2xl text-brand-50 sm:text-3xl">Sign in to the console</h1>
          <p className="mt-1.5 text-sm text-brand-200/65">
            Staff sign-in only. Guests can <Link to="/book" className="text-gold-300 hover:text-gold-200">book a stay</Link> or <Link to="/booking/lookup" className="text-gold-300 hover:text-gold-200">manage a booking</Link>.
          </p>

          <form onSubmit={submit} className="mt-6 rounded-2xl border border-brand-800/80 bg-brand-900/50 p-6 shadow-panel backdrop-blur">
            {error ? (
              <div
                role="alert"
                className="mb-4 rounded-lg border border-rose-500/40 bg-rose-500/10 px-3 py-2 text-xs text-rose-200"
              >
                {error}
              </div>
            ) : null}

            <label className="block text-sm">
              <span className="mb-1 block text-xs text-brand-200/70">Work email</span>
              <input
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full rounded-lg border border-brand-800 bg-brand-950/70 px-3 py-2.5 text-sm text-brand-50 outline-none transition-colors focus:border-gold-500/60"
                placeholder="you@property.com"
              />
            </label>

            <label className="mt-4 block text-sm">
              <span className="mb-1 flex items-center justify-between text-xs text-brand-200/70">
                <span>Password</span>
                <a href="#" className="text-[10px] text-brand-200/50 hover:text-gold-300">Forgot?</a>
              </span>
              <input
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full rounded-lg border border-brand-800 bg-brand-950/70 px-3 py-2.5 text-sm text-brand-50 outline-none transition-colors focus:border-gold-500/60"
                placeholder="••••••••"
              />
            </label>

            <button
              type="submit"
              disabled={submitting}
              className="mt-6 flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-gold-400 to-gold-500 px-4 py-2.5 text-sm font-semibold text-brand-950 transition-all hover:from-gold-300 hover:to-gold-400 disabled:opacity-60"
            >
              {submitting ? (
                <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-brand-950/40 border-t-brand-950" />
              ) : (
                <LogIn size={15} />
              )}
              {submitting ? "Signing in…" : "Sign in"}
            </button>

            <div className="mt-5 border-t border-brand-800/70 pt-4">
              <p className="mb-2 text-[11px] uppercase tracking-wider text-brand-200/50">
                Demo accounts · password <span className="font-mono text-gold-300">{DEMO_PASSWORD}</span>
              </p>
              <div className="flex flex-wrap gap-1.5">
                {DEMO_ACCOUNTS.map((u) => (
                  <button
                    key={u.id}
                    type="button"
                    onClick={() => quickFill(u.email)}
                    className="rounded-full border border-brand-800 bg-brand-950/60 px-2.5 py-1 text-[11px] text-brand-100/85 transition-colors hover:border-gold-500/50 hover:text-gold-300"
                  >
                    {ROLE_LABELS[u.role]}
                  </button>
                ))}
              </div>
            </div>
          </form>

          <p className="mt-6 flex items-center justify-center gap-2 text-center text-xs text-brand-200/50">
            <KeyRound size={12} /> Sessions are role-scoped — the sidebar adapts to your permissions.
          </p>
          <p className="mt-3 text-center text-xs">
            <Link to="/" className="text-brand-200/60 transition-colors hover:text-gold-300">
              ← Back to home <ArrowRight size={10} className="inline" />
            </Link>
          </p>
        </div>
      </section>
    </div>
  );
}
