import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { api, clearToken, getToken, setToken, type Privilege, type User } from "./api";

interface AuthState {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  /** can reports whether the current user holds a privilege. */
  can: (p: Privilege) => boolean;
  /** refresh re-fetches the current user (e.g. after clearing must-change). */
  refresh: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  // On boot, if a token exists, resolve the current user.
  useEffect(() => {
    if (!getToken()) {
      setLoading(false);
      return;
    }
    api
      .me()
      .then(setUser)
      .catch(() => clearToken())
      .finally(() => setLoading(false));
  }, []);

  async function login(email: string, password: string) {
    const res = await api.login(email, password);
    setToken(res.token);
    setUser(res.user);
  }

  function logout() {
    clearToken();
    setUser(null);
  }

  async function refresh() {
    setUser(await api.me());
  }

  function can(p: Privilege): boolean {
    return !!user?.privileges?.includes(p);
  }

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, can, refresh }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
