import { useState, type FormEvent } from "react";
import { KeyRound } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

/**
 * ChangePasswordPage is used both for the forced first-login change (forced) and
 * voluntarily from the sidebar. On a forced change it renders full-screen and,
 * on success, refreshes the user so the app unlocks.
 */
export function ChangePasswordPage({ forced = false }: { forced?: boolean }) {
  const { refresh, logout } = useAuth();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (next !== confirm) {
      setError("New passwords do not match");
      return;
    }
    setBusy(true);
    try {
      await api.changePassword(current, next);
      setDone(true);
      setCurrent("");
      setNext("");
      setConfirm("");
      await refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not change password");
    } finally {
      setBusy(false);
    }
  }

  const form = (
    <Card className={forced ? "w-full max-w-sm" : "max-w-md"}>
      <CardHeader className="space-y-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary text-primary-foreground">
          <KeyRound className="h-5 w-5" />
        </div>
        <div>
          <CardTitle className="text-xl">
            {forced ? "Set a new password" : "Change password"}
          </CardTitle>
          <CardDescription>
            {forced
              ? "Your account was created with a temporary password. Choose a new one to continue."
              : "Update the password for your account."}
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="current">{forced ? "Temporary password" : "Current password"}</Label>
            <Input id="current" type="password" value={current}
              onChange={(e) => setCurrent(e.target.value)} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="next">New password</Label>
            <Input id="next" type="password" value={next} minLength={8}
              onChange={(e) => setNext(e.target.value)} required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirm">Confirm new password</Label>
            <Input id="confirm" type="password" value={confirm} minLength={8}
              onChange={(e) => setConfirm(e.target.value)} required />
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          {done && !forced && <p className="text-sm text-muted-foreground">Password updated.</p>}
          <Button type="submit" className="w-full" disabled={busy}>
            {busy ? "Saving…" : forced ? "Set password & continue" : "Update password"}
          </Button>
          {forced && (
            <Button type="button" variant="ghost" className="w-full" onClick={logout}>
              Sign out
            </Button>
          )}
        </form>
      </CardContent>
    </Card>
  );

  if (forced) {
    return <div className="flex min-h-screen items-center justify-center bg-background px-4">{form}</div>;
  }
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold tracking-tight">Account</h1>
      {form}
    </div>
  );
}
