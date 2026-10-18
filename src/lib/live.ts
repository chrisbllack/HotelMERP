// Live-data bridge: imperative Convex query hook with graceful degradation.
//
// useQuery from convex/react throws during render when the backend is
// unreachable — which would blank the console whenever `convex dev` isn't
// running. This hook issues the query imperatively: on failure it returns
// undefined and the page keeps rendering its local/demo data, so the console
// works in both modes without an error boundary.

import { useEffect, useState } from "react";
import { useConvex, type ConvexReactClient } from "convex/react";

export type QueryFn = Parameters<ConvexReactClient["query"]>[0];

/**
 * Resolve `apiFn(args)` once args is non-null. Returns undefined while loading
 * OR when the backend is unreachable (callers treat undefined as "no live
 * data" and fall back to local state).
 */
export function useConvexQuery<R>(apiFn: QueryFn, args: Record<string, unknown> | null): R | undefined {
  const convex = useConvex();
  // State is keyed by the args so stale results from a previous args set are
  // never returned (and no synchronous setState is needed when skipping).
  const [state, setState] = useState<{ key: string; data: R | undefined }>({ key: "skip", data: undefined });
  const key = args ? JSON.stringify(args) : "skip";

  useEffect(() => {
    let alive = true;
    if (key === "skip") return;
    const parsed = JSON.parse(key) as Record<string, unknown>;
    convex
      .query(apiFn, parsed)
      .then((result) => {
        if (alive) setState({ key, data: result as R });
      })
      .catch(() => {
        if (alive) setState({ key, data: undefined }); // backend offline → callers use fallback data
      });
    return () => {
      alive = false;
    };
  }, [convex, key]); // eslint-disable-line react-hooks/exhaustive-deps

  return state.key === key ? state.data : undefined;
}

/** Imperative mutation helper: resolves to an error message or null. */
export function useConvexCall() {
  const convex = useConvex();
  return async <R,>(apiFn: Parameters<ConvexReactClient["mutation"]>[0], args: Record<string, unknown>): Promise<{ ok: true; data: R } | { ok: false; error: string }> => {
    try {
      const data = (await convex.mutation(apiFn as Parameters<ConvexReactClient["mutation"]>[0], args)) as R;
      return { ok: true, data };
    } catch (err) {
      const message =
        typeof err === "object" && err !== null && "data" in err
          ? String((err as { data: unknown }).data)
          : err instanceof Error
            ? err.message
            : "Request failed";
      return { ok: false, error: message };
    }
  };
}
