import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { useConvex } from "convex/react";
import { ConvexError } from "convex/values";
import { api } from "../convex/_generated/api";
import { users } from "../data/demo";
import type { Role, User } from "../types";

const TOKEN_KEY = "hotelmerp.session";

/** Backend roleKey → console Role (UI mirror of the server RBAC matrix). */
function mapRole(roleKey: string): Role {
  switch (roleKey) {
    case "owner":
    case "admin":
      return "admin";
    case "manager":
      return "general_manager";
    case "finance":
      return "accountant";
    case "hr":
      return "hr";
    case "fnb":
      return "fnb";
    case "front_desk":
    case "guest":
    default:
      return "front_desk";
  }
}

interface AuthState {
  user: User | null;
  /** Server session token — null in local demo mode (backend unreachable). */
  token: string | null;
  /** Backend role key driving the permission matrix. */
  roleKey: string | null;
  permissions: string[];
  /** True when a live Convex session backs this console. */
  isLive: boolean;
  login: (email: string, password: string) => Promise<{ ok: boolean; error?: string }>;
  logout: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

export const DEMO_PASSWORD = "demo1234";

export function AuthProvider({ children }: { children: ReactNode }) {
  const convex = useConvex();
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [roleKey, setRoleKey] = useState<string | null>(null);
  const [permissions, setPermissions] = useState<string[]>([]);

  // Restore a persisted session on reload (validates against the backend).
  useEffect(() => {
    const stored = localStorage.getItem(TOKEN_KEY);
    if (!stored) return;
    let alive = true;
    convex
      .query(api.authz.me, { token: stored })
      .then((session) => {
        if (!alive) return;
        if (session) {
          setToken(stored);
          setRoleKey(session.roleKey);
          setPermissions(session.permissions);
          setUser({
            id: session.userId,
            name: session.displayName || session.name,
            email: session.email,
            role: mapRole(session.roleKey),
            status: "active",
          });
        } else {
          localStorage.removeItem(TOKEN_KEY);
        }
      })
      .catch(() => {
        // Backend unreachable — clear the stale token; demo login still works.
        if (alive) localStorage.removeItem(TOKEN_KEY);
      });
    return () => {
      alive = false;
    };
  }, [convex]);

  const login = useCallback(
    async (email: string, password: string): Promise<{ ok: boolean; error?: string }> => {
      // 1) Preferred: server-side session (PBKDF2 + RBAC, all checks enforced
      //    downstream by requireAuth on every privileged surface).
      try {
        const session = await convex.mutation(api.authz.login, { email, password });
        localStorage.setItem(TOKEN_KEY, session.token);
        setToken(session.token);
        setRoleKey(session.roleKey);
        setPermissions(session.permissions);
        setUser({
          id: session.userId,
          name: session.displayName || session.name,
          email: session.email,
          role: mapRole(session.roleKey),
          status: "active",
        });
        return { ok: true };
      } catch (err) {
        // A ConvexError is a definitive auth answer from the server.
        if (err instanceof ConvexError) {
          return { ok: false, error: String(err.data) };
        }
        // Otherwise (backend offline) fall through to local demo mode.
      }

      // 2) Local demo fallback — keeps the console operable offline.
      if (password !== DEMO_PASSWORD) return { ok: false, error: "Incorrect password. Demo password is demo1234." };
      const found = users.find((u) => u.email.toLowerCase() === email.trim().toLowerCase());
      if (!found) {
        return { ok: false, error: "No account with that email. Try one of the demo accounts below." };
      }
      localStorage.removeItem(TOKEN_KEY);
      setToken(null);
      setRoleKey(null);
      setPermissions([]);
      setUser(found);
      return { ok: true };
    },
    [convex],
  );

  const logout = useCallback(() => {
    if (token) {
      void convex.mutation(api.authz.logout, { token }).catch(() => undefined);
    }
    localStorage.removeItem(TOKEN_KEY);
    setToken(null);
    setRoleKey(null);
    setPermissions([]);
    setUser(null);
  }, [convex, token]);

  const value = useMemo<AuthState>(
    () => ({ user, token, roleKey, permissions, isLive: token !== null, login, logout }),
    [user, token, roleKey, permissions, login, logout],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
