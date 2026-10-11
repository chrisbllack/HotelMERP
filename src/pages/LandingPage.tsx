import { Link } from "react-router-dom";
import {
  ArrowRight, ArrowUpRight, BedDouble, Calendar, Check, ChevronRight, Clock, Coffee,
  ConciergeBell, Crown, MapPin, Phone, Plane, Sparkles, Star, Users, UtensilsCrossed,
  Waves, Wifi, Wine,
} from "lucide-react";

// ─── Real hotel imagery (Unsplash CDN, free to use) ──────────────────────────
const HERO_IMAGE =
  "https://images.unsplash.com/photo-1582719508461-905c673771fd?ixlib=rb-4.1.0&auto=format&fit=crop&w=2400&q=80";
const SECONDARY_HERO =
  "https://images.unsplash.com/photo-1542314831-068cd1dbfeeb?ixlib=rb-4.1.0&auto=format&fit=crop&w=1200&q=80";

const ROOMS: Array<{
  name: string;
  tagline: string;
  price: number;
  size: string;
  bed: string;
  guests: number;
  image: string;
  features: string[];
}> = [
  {
    name: "Deluxe King",
    tagline: "City-view sanctuary with king bed, marble bath, and curated minibar.",
    price: 95000,
    size: "38 m²",
    bed: "1 King",
    guests: 2,
    image:
      "https://images.unsplash.com/photo-1771775529138-a7a20ba7e032?ixlib=rb-4.1.0&auto=format&fit=crop&w=1400&q=80",
    features: ["Free Wi-Fi 1 Gbps", "Nespresso machine", "Rain shower", "55\" Smart TV"],
  },
  {
    name: "Executive Suite",
    tagline: "Separate living room, work desk and panoramic Atlantic-facing windows.",
    price: 165000,
    size: "62 m²",
    bed: "1 King + sofa",
    guests: 3,
    image:
      "https://images.unsplash.com/photo-1777169794972-12095816073b?ixlib=rb-4.1.0&auto=format&fit=crop&w=1400&q=80",
    features: ["Lounge access", "Espresso bar", "Walk-in closet", "Soaking tub"],
  },
  {
    name: "Royal Penthouse",
    tagline: "Top-floor residence with private terrace, plunge pool and butler service.",
    price: 420000,
    size: "148 m²",
    bed: "2 Kings",
    guests: 4,
    image:
      "https://images.unsplash.com/photo-1788927796219-35f6c27e6422?ixlib=rb-4.1.0&auto=format&fit=crop&w=1400&q=80",
    features: ["Butler service", "Plunge pool", "Private dining", "Limo airport transfer"],
  },
];

const AMENITIES: Array<{ icon: typeof Waves; name: string; description: string; image?: string }> = [
  { icon: Waves, name: "Rooftop infinity pool", description: "Heated year-round with Lagos skyline view" },
  { icon: UtensilsCrossed, name: "Three signature restaurants", description: "Lagoon grill · Atrium · Sky-bar" },
  { icon: Sparkles, name: "Aurora Spa & Hammam", description: "Six treatment rooms and a marble hammam" },
  { icon: Wine, name: "Cigar & whisky lounge", description: "Curated cellar with 240+ labels" },
  { icon: ConciergeBell, name: "24-hour concierge", description: "Tours, transfers, and theatre bookings" },
  { icon: Plane, name: "Helipad & private jets", description: "Direct transfers from MMA2 in twelve minutes" },
];

const EXPERIENCES: Array<{ title: string; blurb: string; image: string }> = [
  {
    title: "Lekki conservation morning",
    blurb: "A guided canopy-walk before breakfast, returning to a chef's table on the lagoon deck.",
    image:
      "https://images.unsplash.com/photo-1542314831-068cd1dbfeeb?ixlib=rb-4.1.0&auto=format&fit=crop&w=900&q=80",
  },
  {
    title: "Atelier cooking class",
    blurb: "Spend an afternoon with Chef Tolu in the Atrium's open kitchen — a Lagos tasting menu you compose.",
    image:
      "https://images.unsplash.com/photo-1763207291832-819499e261dd?ixlib=rb-4.1.0&auto=format&fit=crop&w=900&q=80",
  },
  {
    title: "Sunset sail on the lagoon",
    blurb: "Two hours aboard our mahogany ketch with a champagne service and a private musician.",
    image:
      "https://images.unsplash.com/photo-1502920917128-1aa500764cbd?ixlib=rb-4.1.0&auto=format&fit=crop&w=900&q=80",
  },
];

const TESTIMONIALS: Array<{ quote: string; author: string; context: string }> = [
  {
    quote:
      "The Atrium's tasting menu on the second night was one of the best meals I've had in West Africa — attentive without being performative.",
    author: "A frequent guest",
    context: "Stayed in the Executive Suite, October",
  },
  {
    quote:
      "Our weekend was made by the rooftop pool in the late afternoon. Service remembered the children's names by the second day.",
    author: "A weekend visitor",
    context: "Family of four, August",
  },
  {
    quote:
      "The concierge moved a 6:30am airport run forward by twenty minutes without a fuss. The kind of quiet competence you hope for.",
    author: "A business traveller",
    context: "Five stays in 2024",
  },
];

function naira(value: number) {
  return new Intl.NumberFormat("en-NG", { style: "currency", currency: "NGN", maximumFractionDigits: 0 }).format(value);
}

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-brand-950 text-brand-50">
      {/* Skip link for keyboard users */}
      <a href="#main" className="skip-link">Skip to content</a>

      {/* Top bar */}
      <header className="absolute inset-x-0 top-0 z-30">
        <div className="mx-auto flex h-20 max-w-7xl items-center justify-between px-5 lg:px-10">
          <Link to="/" className="flex items-center gap-2.5">
            <div className="flex h-10 w-10 items-center justify-center rounded-md bg-gradient-to-br from-gold-400 to-gold-600 font-display text-lg text-brand-950 shadow-lg ring-1 ring-white/10">
              G
            </div>
            <div className="leading-tight">
              <p className="font-display text-base tracking-wide text-white">Grand Palm</p>
              <p className="text-[10px] uppercase tracking-[0.25em] text-gold-300/80">Hotel Lagos · Est. 1998</p>
            </div>
          </Link>
          <nav className="hidden items-center gap-7 text-sm text-white/80 lg:flex">
            <a href="#rooms" className="transition-colors hover:text-gold-300">Rooms & suites</a>
            <a href="#dining" className="transition-colors hover:text-gold-300">Dining</a>
            <a href="#wellness" className="transition-colors hover:text-gold-300">Wellness</a>
            <a href="#experiences" className="transition-colors hover:text-gold-300">Experiences</a>
            <a href="#location" className="transition-colors hover:text-gold-300">Location</a>
            <Link to="/platform" className="rounded-full border border-gold-400/40 px-2.5 py-1 text-[11px] font-medium uppercase tracking-wider text-gold-200 transition-colors hover:bg-gold-500/10">
              Run your hotel on this →
            </Link>
          </nav>
          <div className="flex items-center gap-3">
            <a
              href="tel:+23414612900"
              className="hidden items-center gap-2 text-sm text-white/80 transition-colors hover:text-gold-300 lg:flex"
            >
              <Phone size={14} /> +234 1 461 2900
            </a>
            <Link
              to="/auth?returnTo=%2Fdashboard"
              className="rounded-md border border-white/20 px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-white/10"
            >
              Staff
            </Link>
            <Link
              to="/book"
              className="flex items-center gap-1.5 rounded-md bg-gradient-to-r from-gold-400 to-gold-500 px-4 py-2 text-sm font-semibold text-brand-950 shadow-md transition-all hover:shadow-lg"
            >
              Reserve <ArrowRight size={14} />
            </Link>
          </div>
        </div>
      </header>

      <main id="main">
        {/* Hero — full-bleed image with editorial overlay */}
        <section className="relative">
          <div className="relative h-[88vh] min-h-[640px] w-full overflow-hidden">
            <img
              src={HERO_IMAGE}
              alt="Grand Palm Hotel Lagos at dusk, viewed from the lagoon"
              className="h-full w-full object-cover"
            />
            <div className="absolute inset-0 bg-gradient-to-b from-black/40 via-black/20 to-black/85" aria-hidden="true" />
            <div className="absolute inset-0 bg-gradient-to-r from-black/55 via-transparent to-transparent" aria-hidden="true" />

            {/* Hero content */}
            <div className="absolute inset-0 flex flex-col justify-end">
              <div className="mx-auto w-full max-w-7xl px-5 pb-16 lg:px-10 lg:pb-20">
                <div className="max-w-3xl">
                  <span className="inline-flex items-center gap-2 rounded-full border border-gold-400/40 bg-black/35 px-3.5 py-1.5 text-[11px] font-medium uppercase tracking-[0.2em] text-gold-200 backdrop-blur">
                    <Star size={11} className="fill-gold-300 text-gold-300" /> Five-star lagoonfront · Lagos Island
                  </span>
                  <h1 className="mt-6 font-display text-5xl leading-[1.05] text-white sm:text-6xl lg:text-7xl">
                    Stay where the city <br className="hidden sm:block" />
                    <em className="not-italic bg-gradient-to-r from-gold-200 via-gold-300 to-gold-500 bg-clip-text text-transparent">
                      exhales
                    </em>
                    .
                  </h1>
                  <p className="mt-6 max-w-xl text-lg text-white/85">
                    Twenty-six years of Lagos hospitality, reimagined. Fifty rooms and suites above
                    the lagoon, three restaurants, a rooftop pool, and a spa built around a marble hammam.
                  </p>
                  <div className="mt-8 flex flex-wrap items-center gap-3">
                    <Link
                      to="/book"
                      className="group flex items-center gap-2 rounded-md bg-gradient-to-r from-gold-400 to-gold-500 px-6 py-3.5 text-sm font-semibold text-brand-950 shadow-xl transition-all hover:from-gold-300 hover:to-gold-400"
                    >
                      Reserve a room
                      <ArrowRight size={16} className="transition-transform group-hover:translate-x-0.5" />
                    </Link>
                    <a
                      href="#rooms"
                      className="flex items-center gap-2 rounded-md border border-white/30 bg-white/5 px-6 py-3.5 text-sm font-medium text-white backdrop-blur transition-colors hover:bg-white/15"
                    >
                      Explore rooms
                    </a>
                  </div>
                  <div className="mt-10 flex flex-wrap items-center gap-x-8 gap-y-3 text-xs text-white/60">
                    <span className="flex items-center gap-2"><Check size={13} className="text-gold-300" /> Free cancellation up to 24h</span>
                    <span className="flex items-center gap-2"><Check size={13} className="text-gold-300" /> No prepayment needed</span>
                    <span className="flex items-center gap-2"><Check size={13} className="text-gold-300" /> Best rate on our site</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Sticky booking bar */}
          <div className="relative -mt-10 z-10 mx-auto max-w-6xl px-5 lg:px-0">
            <div className="rounded-2xl border border-white/10 bg-brand-900/85 p-2 shadow-2xl backdrop-blur">
              <form
                className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-5"
                onSubmit={(e) => {
                  e.preventDefault();
                  window.location.href = "/book";
                }}
              >
                <Field icon={<BedDouble size={16} />} label="Room type" defaultValue="Any room" />
                <Field icon={<Calendar size={16} />} label="Check-in" type="date" defaultValue="2026-10-20" />
                <Field icon={<Calendar size={16} />} label="Check-out" type="date" defaultValue="2026-10-23" />
                <Field icon={<Users size={16} />} label="Guests" defaultValue="2 adults" />
                <button
                  type="submit"
                  className="flex items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-gold-400 to-gold-500 px-4 py-3 text-sm font-semibold text-brand-950 transition-all hover:from-gold-300 hover:to-gold-400 lg:rounded-lg"
                >
                  Check availability <ArrowRight size={15} />
                </button>
              </form>
            </div>
          </div>
        </section>

        {/* Intro band */}
        <section className="border-y border-brand-800/60 bg-brand-900/30 py-14">
          <div className="mx-auto grid max-w-7xl gap-10 px-5 lg:grid-cols-3 lg:px-10">
            <div>
              <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Welcome to Grand Palm</p>
              <h2 className="mt-3 font-display text-2xl text-brand-50 sm:text-3xl">
                A Lagos address that has hosted heads of state, footballers, and poets — all treated like family.
              </h2>
            </div>
            <p className="text-sm leading-relaxed text-brand-100/80 sm:col-span-2 sm:text-base">
              We are a fifty-room, independently-owned hotel on the western edge of Lagos Island. Our
              colleagues have been here an average of twelve years. Our coffee is roasted for us in
              Ibadan, our soap is milled in Abeokuta, and our bath linens come from a family weaver in
              Antakya. The result is a hotel that feels like a private residence — and the kind of place
              where the same guest is greeted by name on their third visit.
            </p>
          </div>
        </section>

        {/* Rooms */}
        <section id="rooms" className="bg-brand-950 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div className="max-w-2xl">
                <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Rooms & suites</p>
                <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">
                  Three signature layouts. Fifty rooms in total.
                </h2>
              </div>
              <p className="max-w-md text-sm text-brand-100/70">
                Every room shares the same hand-finished details — Egyptian cotton, locally-milled
                soap, and a minibar stocked with small-batch producers.
              </p>
            </div>

            <div className="mt-10 grid gap-6 lg:grid-cols-3">
              {ROOMS.map((room) => (
                <article
                  key={room.name}
                  className="group overflow-hidden rounded-2xl border border-brand-800/70 bg-brand-900/40 shadow-panel transition-all hover:border-gold-500/50 hover:shadow-2xl"
                >
                  <div className="relative aspect-[4/3] overflow-hidden">
                    <img
                      src={room.image}
                      alt={`${room.name} at Grand Palm Hotel Lagos`}
                      className="h-full w-full object-cover transition-transform duration-700 group-hover:scale-105"
                    />
                    <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 via-black/20 to-transparent p-4">
                      <div className="flex items-end justify-between">
                        <div>
                          <p className="text-[10px] uppercase tracking-[0.2em] text-gold-300">From</p>
                          <p className="font-display text-xl text-white">{naira(room.price)}</p>
                          <p className="text-[10px] text-white/60">per night, includes taxes</p>
                        </div>
                        <span className="rounded-full border border-white/30 bg-white/10 px-2 py-0.5 text-[10px] text-white/80 backdrop-blur">
                          {room.size} · sleeps {room.guests}
                        </span>
                      </div>
                    </div>
                  </div>
                  <div className="p-5">
                    <div className="flex items-center justify-between">
                      <h3 className="font-display text-xl text-brand-50">{room.name}</h3>
                      <span className="text-[10px] uppercase tracking-wider text-brand-200/50">{room.bed}</span>
                    </div>
                    <p className="mt-2 text-sm leading-relaxed text-brand-100/75">{room.tagline}</p>
                    <ul className="mt-4 grid grid-cols-2 gap-x-3 gap-y-1.5 text-[11px] text-brand-200/70">
                      {room.features.map((f) => (
                        <li key={f} className="flex items-start gap-1.5">
                          <Check size={11} className="mt-0.5 flex-none text-gold-400" /> {f}
                        </li>
                      ))}
                    </ul>
                    <Link
                      to="/book"
                      className="mt-5 inline-flex items-center gap-1.5 text-xs font-medium text-gold-300 transition-colors hover:text-gold-200"
                    >
                      Reserve this room <ArrowUpRight size={13} />
                    </Link>
                  </div>
                </article>
              ))}
            </div>
          </div>
        </section>

        {/* Amenities — image + text grid */}
        <section className="border-t border-brand-800/60 bg-brand-900/30 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="grid gap-12 lg:grid-cols-12 lg:items-center">
              <div className="lg:col-span-5">
                <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">The property</p>
                <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">
                  Designed to be lived in, not photographed.
                </h2>
                <p className="mt-5 text-sm leading-relaxed text-brand-100/80 sm:text-base">
                  Six amenities, deliberately chosen. We don't have a bowling alley, a waterpark, or a
                  rooftop helipad that the city doesn't need. We have a hammam we believe in, a coffee
                  programme we taste every fortnight, and a gym that opens at 5am.
                </p>
              </div>
              <div className="lg:col-span-7">
                <div className="grid gap-4 sm:grid-cols-2">
                  {AMENITIES.map((a) => (
                    <div
                      key={a.name}
                      className="flex gap-4 rounded-xl border border-brand-800/70 bg-brand-950/60 p-4 transition-colors hover:border-gold-500/40"
                    >
                      <div className="flex h-10 w-10 flex-none items-center justify-center rounded-lg bg-gold-500/10 text-gold-400 ring-1 ring-gold-500/30">
                        <a.icon size={18} />
                      </div>
                      <div>
                        <p className="font-medium text-brand-50">{a.name}</p>
                        <p className="mt-0.5 text-xs text-brand-100/65">{a.description}</p>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Dining — magazine layout */}
        <section id="dining" className="bg-brand-950 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="grid gap-10 lg:grid-cols-2">
              <div className="relative overflow-hidden rounded-2xl">
                <img
                  src="https://images.unsplash.com/photo-1785845506893-70768a28ba44?ixlib=rb-4.1.0&auto=format&fit=crop&w=1600&q=80"
                  alt="Atrium restaurant interior at Grand Palm Hotel Lagos"
                  className="h-full w-full object-cover"
                />
                <div className="absolute inset-0 bg-gradient-to-t from-black/70 via-transparent to-transparent" />
                <div className="absolute bottom-0 p-8">
                  <span className="text-[10px] uppercase tracking-[0.3em] text-gold-300">The Atrium</span>
                  <h3 className="mt-2 font-display text-3xl text-white">A Lagos tasting menu, ten courses long.</h3>
                  <p className="mt-3 max-w-md text-sm text-white/80">
                    Chef Tolu Ighodaro composes a seasonal menu that draws on the coastal markets of
                    Lagos, Port Harcourt and Cotonou. Pair with the sommelier's selection of West African wines.
                  </p>
                </div>
              </div>
              <div className="space-y-4">
                <div className="relative overflow-hidden rounded-2xl">
                  <img
                    src="https://images.unsplash.com/photo-1763207291832-819499e261dd?ixlib=rb-4.1.0&auto=format&fit=crop&w=1200&q=80"
                    alt="Breakfast service at Grand Palm"
                    className="h-48 w-full object-cover"
                  />
                  <div className="absolute inset-0 bg-gradient-to-t from-black/65 to-transparent" />
                  <div className="absolute bottom-0 p-5">
                    <p className="text-[10px] uppercase tracking-[0.2em] text-gold-300">Lagoon Grill</p>
                    <p className="mt-1 font-display text-xl text-white">Sunday brunch, lagoon-side</p>
                  </div>
                </div>
                <div className="relative overflow-hidden rounded-2xl">
                  <img
                    src="https://images.unsplash.com/photo-1745327883290-1e9c6447b938?ixlib=rb-4.1.0&auto=format&fit=crop&w=1200&q=80"
                    alt="Sky bar at dusk"
                    className="h-48 w-full object-cover"
                  />
                  <div className="absolute inset-0 bg-gradient-to-t from-black/65 to-transparent" />
                  <div className="absolute bottom-0 p-5">
                    <p className="text-[10px] uppercase tracking-[0.2em] text-gold-300">Sky-bar</p>
                    <p className="mt-1 font-display text-xl text-white">Sunset hour, six until nine</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Wellness */}
        <section id="wellness" className="border-y border-brand-800/60 bg-brand-900/30 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="grid gap-10 lg:grid-cols-2 lg:items-center">
              <div className="order-2 lg:order-1">
                <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Aurora Spa</p>
                <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">
                  A marble hammam, six treatment rooms, and a roof for quiet.
                </h2>
                <p className="mt-5 text-sm leading-relaxed text-brand-100/80 sm:text-base">
                  Treatments blend West African botanicals with classical technique. Our hammam is
                  built from a single block of Carrara, and is kept at 42°C year-round.
                </p>
                <div className="mt-6 flex flex-wrap gap-3">
                  <span className="flex items-center gap-1.5 rounded-full border border-brand-700/60 bg-brand-950/60 px-3 py-1.5 text-[11px] text-brand-100/80">
                    <Coffee size={12} className="text-gold-400" /> Hammam rituals
                  </span>
                  <span className="flex items-center gap-1.5 rounded-full border border-brand-700/60 bg-brand-950/60 px-3 py-1.5 text-[11px] text-brand-100/80">
                    <Waves size={12} className="text-gold-400" /> Hydrotherapy
                  </span>
                  <span className="flex items-center gap-1.5 rounded-full border border-brand-700/60 bg-brand-950/60 px-3 py-1.5 text-[11px] text-brand-100/80">
                    <Sparkles size={12} className="text-gold-400" /> Shear butter rituals
                  </span>
                </div>
                <Link
                  to="/book"
                  className="mt-8 inline-flex items-center gap-2 text-sm font-medium text-gold-300 transition-colors hover:text-gold-200"
                >
                  Reserve a spa treatment <ChevronRight size={14} />
                </Link>
              </div>
              <div className="order-1 lg:order-2">
                <div className="relative overflow-hidden rounded-2xl">
                  <img
                    src="https://images.unsplash.com/photo-1600334129128-685c5582fd35?ixlib=rb-4.1.0&auto=format&fit=crop&w=1400&q=80"
                    alt="Aurora Spa treatment room"
                    className="h-[460px] w-full object-cover"
                  />
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Experiences */}
        <section id="experiences" className="bg-brand-950 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div>
                <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Experiences</p>
                <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">A few things our guests remember.</h2>
              </div>
              <p className="max-w-md text-sm text-brand-100/70">
                Curated by our concierge, included or available à la carte. Tell us what you'd like
                and we'll arrange it.
              </p>
            </div>
            <div className="mt-10 grid gap-6 md:grid-cols-3">
              {EXPERIENCES.map((exp) => (
                <article
                  key={exp.title}
                  className="group overflow-hidden rounded-2xl border border-brand-800/70 bg-brand-900/40 transition-all hover:border-gold-500/40"
                >
                  <div className="aspect-[4/3] overflow-hidden">
                    <img
                      src={exp.image}
                      alt={exp.title}
                      className="h-full w-full object-cover transition-transform duration-700 group-hover:scale-105"
                    />
                  </div>
                  <div className="p-5">
                    <h3 className="font-display text-lg text-brand-50">{exp.title}</h3>
                    <p className="mt-1.5 text-sm text-brand-100/70">{exp.blurb}</p>
                  </div>
                </article>
              ))}
            </div>
          </div>
        </section>

        {/* Testimonials */}
        <section className="border-t border-brand-800/60 bg-brand-900/30 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">From the guest book</p>
            <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">Quietly recommended.</h2>
            <div className="mt-10 grid gap-6 md:grid-cols-3">
              {TESTIMONIALS.map((t) => (
                <figure
                  key={t.author}
                  className="rounded-2xl border border-brand-800/70 bg-brand-950/60 p-6"
                >
                  <Crown className="text-gold-400/70" size={20} />
                  <blockquote className="mt-4 font-display text-lg leading-snug text-brand-50 sm:text-xl">
                    "{t.quote}"
                  </blockquote>
                  <figcaption className="mt-5 border-t border-brand-800/70 pt-4 text-xs text-brand-200/65">
                    <p className="font-medium text-brand-100">{t.author}</p>
                    <p>{t.context}</p>
                  </figcaption>
                </figure>
              ))}
            </div>
          </div>
        </section>

        {/* Location */}
        <section id="location" className="bg-brand-950 py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-10">
            <div className="overflow-hidden rounded-2xl border border-brand-800/70 bg-brand-900/40">
              <div className="grid gap-0 lg:grid-cols-2">
                <div className="relative aspect-[4/3] lg:aspect-auto">
                  <img
                    src={SECONDARY_HERO}
                    alt="Lagos waterfront at sunset"
                    className="h-full w-full object-cover"
                  />
                </div>
                <div className="p-8 sm:p-10">
                  <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Find us</p>
                  <h2 className="mt-3 font-display text-3xl text-brand-50 sm:text-4xl">Lagos Island, by the lagoon.</h2>
                  <div className="mt-6 space-y-4 text-sm text-brand-100/80">
                    <p className="flex items-start gap-3">
                      <MapPin size={16} className="mt-0.5 flex-none text-gold-400" />
                      <span>
                        <span className="block font-medium text-brand-50">Grand Palm Hotel Lagos</span>
                        <span className="block text-brand-100/70">1 Lake Drive, Off Awolowo Road, Ikoyi, Lagos</span>
                      </span>
                    </p>
                    <p className="flex items-start gap-3">
                      <Clock size={16} className="mt-0.5 flex-none text-gold-400" />
                      <span>
                        <span className="block font-medium text-brand-50">Check-in / out</span>
                        <span className="block text-brand-100/70">From 14:00 · until 12:00 (late check-out on request)</span>
                      </span>
                    </p>
                    <p className="flex items-start gap-3">
                      <Phone size={16} className="mt-0.5 flex-none text-gold-400" />
                      <span>
                        <span className="block font-medium text-brand-50">Reservations</span>
                        <span className="block text-brand-100/70">+234 1 461 2900 · reservations@grandpalm.ng</span>
                      </span>
                    </p>
                    <p className="flex items-start gap-3">
                      <Plane size={16} className="mt-0.5 flex-none text-gold-400" />
                      <span>
                        <span className="block font-medium text-brand-50">From the airport</span>
                        <span className="block text-brand-100/70">MMA2 — 35 minutes · Muritala International — 50 minutes</span>
                      </span>
                    </p>
                  </div>
                  <div className="mt-8 flex flex-wrap gap-3">
                    <Link
                      to="/book"
                      className="inline-flex items-center gap-2 rounded-md bg-gradient-to-r from-gold-400 to-gold-500 px-5 py-2.5 text-sm font-semibold text-brand-950 transition-all hover:from-gold-300 hover:to-gold-400"
                    >
                      Reserve a room <ArrowRight size={15} />
                    </Link>
                    <a
                      href="https://maps.google.com/?q=Grand+Palm+Hotel+Lagos"
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-2 rounded-md border border-brand-700 px-5 py-2.5 text-sm font-medium text-brand-100 transition-colors hover:border-gold-500/40 hover:text-brand-50"
                    >
                      Open in maps <ArrowUpRight size={14} />
                    </a>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Final CTA */}
        <section className="border-t border-brand-800/60 bg-gradient-to-b from-brand-900/30 to-brand-950 py-20">
          <div className="mx-auto max-w-3xl px-5 text-center">
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Reserve direct</p>
            <h2 className="mt-4 font-display text-3xl text-brand-50 sm:text-4xl">
              We hold ten per cent of every room for guests who book with us.
            </h2>
            <p className="mt-4 text-sm text-brand-100/75 sm:text-base">
              Best rate, free cancellation up to 24 hours before arrival, and a complimentary upgrade
              when one is available.
            </p>
            <Link
              to="/book"
              className="mt-8 inline-flex items-center gap-2 rounded-md bg-gradient-to-r from-gold-400 to-gold-500 px-7 py-3.5 text-sm font-semibold text-brand-950 shadow-xl transition-all hover:from-gold-300 hover:to-gold-400"
            >
              Reserve a room <ArrowRight size={16} />
            </Link>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="border-t border-brand-800/60 bg-brand-950 py-14">
        <div className="mx-auto grid max-w-7xl gap-10 px-5 sm:grid-cols-2 lg:grid-cols-4 lg:px-10">
          <div>
            <div className="flex items-center gap-2.5">
              <div className="flex h-9 w-9 items-center justify-center rounded-md bg-gradient-to-br from-gold-400 to-gold-600 font-display text-brand-950">
                G
              </div>
              <div className="leading-tight">
                <p className="font-display text-sm text-brand-50">Grand Palm</p>
                <p className="text-[10px] uppercase tracking-widest text-gold-300/80">Hotel Lagos</p>
              </div>
            </div>
            <p className="mt-4 text-xs leading-relaxed text-brand-100/65">
              An independently-owned hotel since 1998. A member of Leading Hotels of the World.
            </p>
          </div>
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Stay</p>
            <ul className="mt-3 space-y-2 text-sm text-brand-100/80">
              <li><a href="#rooms" className="hover:text-gold-300">Rooms & suites</a></li>
              <li><Link to="/book" className="hover:text-gold-300">Reservations</Link></li>
              <li><a href="#dining" className="hover:text-gold-300">Dining</a></li>
              <li><a href="#wellness" className="hover:text-gold-300">Aurora Spa</a></li>
            </ul>
          </div>
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Contact</p>
            <ul className="mt-3 space-y-2 text-sm text-brand-100/80">
              <li>+234 1 461 2900</li>
              <li>reservations@grandpalm.ng</li>
              <li>1 Lake Drive, Ikoyi, Lagos</li>
            </ul>
          </div>
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-gold-400/80">Staff</p>
            <ul className="mt-3 space-y-2 text-sm text-brand-100/80">
              <li><Link to="/auth?returnTo=%2Fdashboard" className="hover:text-gold-300">Sign in to the console</Link></li>
              <li><Link to="/book/lookup" className="hover:text-gold-300">Manage a reservation</Link></li>
            </ul>
            <p className="mt-4 text-[10px] text-brand-200/40">
              Demo password: <span className="font-mono text-gold-300">demo1234</span>
            </p>
          </div>
        </div>
        <div className="mx-auto mt-10 flex max-w-7xl flex-col items-center justify-between gap-3 border-t border-brand-800/60 px-5 pt-6 text-[11px] text-brand-200/50 sm:flex-row lg:px-10">
          <p>© 2026 Grand Palm Hotel Lagos · InfinityPlus Solutions & Services Ltd.</p>
          <p className="flex items-center gap-2">
            <Wifi size={11} /> Free 1 Gbps Wi-Fi throughout
          </p>
        </div>
      </footer>
    </div>
  );
}

function Field({
  icon, label, type = "text", defaultValue,
}: { icon: React.ReactNode; label: string; type?: string; defaultValue?: string }) {
  return (
    <label className="group flex items-center gap-3 rounded-lg border border-transparent bg-brand-950/40 px-3 py-2.5 transition-colors hover:border-brand-700">
      <span className="text-gold-400">{icon}</span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-[10px] font-semibold uppercase tracking-wider text-brand-200/50">{label}</span>
        <input
          type={type}
          defaultValue={defaultValue}
          className="w-full bg-transparent text-sm text-brand-50 outline-none placeholder:text-brand-200/30"
        />
      </span>
    </label>
  );
}
