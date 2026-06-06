import { useEffect, useState, type ReactNode } from "react";
import { NavLink } from "react-router-dom";
import { Files, HardDrive, Plug, Users, Shield, KeyRound, KeySquare, Moon, Sun, LogOut } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/lib/auth";
import type { Privilege } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Logo } from "@/components/Logo";

// Each nav item is shown only if the user holds its required privilege.
const nav: { to: string; label: string; icon: typeof Files; end: boolean; priv: Privilege }[] = [
  { to: "/", label: "Files", icon: Files, end: true, priv: "files.view" },
  { to: "/pool", label: "Storage Pool", icon: HardDrive, end: false, priv: "files.view" },
  { to: "/keys", label: "API Keys", icon: KeySquare, end: false, priv: "files.view" },
  { to: "/providers", label: "Providers", icon: Plug, end: false, priv: "providers.manage" },
  { to: "/users", label: "Users", icon: Users, end: false, priv: "users.manage" },
  { to: "/roles", label: "Roles", icon: Shield, end: false, priv: "roles.manage" },
];

function useTheme() {
  const [light, setLight] = useState(() => localStorage.getItem("unidisk_theme") === "light");
  useEffect(() => {
    document.documentElement.classList.toggle("light", light);
    localStorage.setItem("unidisk_theme", light ? "light" : "dark");
  }, [light]);
  return { light, toggle: () => setLight((v) => !v) };
}

export function AppLayout({ children }: { children: ReactNode }) {
  const { user, logout, can } = useAuth();
  const { light, toggle } = useTheme();
  const visibleNav = nav.filter((item) => can(item.priv));

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <aside className="flex w-60 shrink-0 flex-col border-r">
        <div className="flex h-14 items-center gap-2 border-b px-5">
          <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <Logo className="h-4 w-4" />
          </div>
          <span className="text-sm font-semibold tracking-tight">UniDisk</span>
        </div>

        <nav className="flex-1 space-y-1 p-3">
          {visibleNav.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-accent text-accent-foreground"
                    : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
                )
              }
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </NavLink>
          ))}
        </nav>

        <div className="space-y-2 border-t p-3">
          <div className="flex items-center justify-between px-1">
            <div className="min-w-0">
              <p className="truncate text-xs font-medium" title={user?.email}>
                {user?.email}
              </p>
              {user?.role_name && (
                <p className="text-[11px] text-muted-foreground">{user.role_name}</p>
              )}
            </div>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={toggle} title="Toggle theme">
              {light ? <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4" />}
            </Button>
          </div>
          <NavLink
            to="/account"
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
              )
            }
          >
            <KeyRound className="h-4 w-4" />
            Change password
          </NavLink>
          <Button variant="outline" size="sm" className="w-full justify-start" onClick={logout}>
            <LogOut className="h-4 w-4" />
            Sign out
          </Button>
        </div>
      </aside>

      <main className="flex-1 overflow-y-auto">
        <div className="animate-page-in mx-auto max-w-5xl px-8 py-8">{children}</div>
      </main>
    </div>
  );
}
