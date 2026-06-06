import { useEffect, useState } from "react";
import { api, ApiError, type ProviderDescriptor } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

/**
 * AddProviderDialog drives the "choose provider → enter credentials → verify →
 * add" flow. The credential form is rendered entirely from the provider's
 * declared schema, so new providers need no changes here.
 */
export function AddProviderDialog({ onAdded }: { onAdded: () => void }) {
  const [open, setOpen] = useState(false);
  const [providers, setProviders] = useState<ProviderDescriptor[]>([]);
  const [selected, setSelected] = useState<ProviderDescriptor | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (open && providers.length === 0) {
      api.providers().then(setProviders).catch(() => setError("Could not load providers"));
    }
  }, [open, providers.length]);

  function reset() {
    setSelected(null);
    setValues({});
    setDisplayName("");
    setError(null);
  }

  async function submit() {
    if (!selected) return;
    setError(null);
    setBusy(true);
    try {
      await api.addAccount(selected.name, displayName, values);
      setOpen(false);
      reset();
      onAdded();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Verification failed");
    } finally {
      setBusy(false);
    }
  }

  // Connect flow: fetch the provider consent URL and navigate there. Google
  // redirects back to /providers after the account is created server-side.
  async function connect() {
    if (!selected) return;
    setError(null);
    setBusy(true);
    try {
      const { url } = await api.oauthStart(selected.name);
      window.location.href = url;
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not start connection");
      setBusy(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (!o) reset();
      }}
    >
      <DialogTrigger asChild>
        <Button>Add provider</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Connect a storage provider</DialogTitle>
          <DialogDescription>
            {selected
              ? "Enter the credentials. They're verified before the account is added."
              : "Choose a provider to connect to your pool."}
          </DialogDescription>
        </DialogHeader>

        {!selected ? (
          <div className="grid gap-2">
            {providers.map((p) => (
              <button
                key={p.name}
                onClick={() => setSelected(p)}
                className="flex items-center justify-between rounded-md border px-4 py-3 text-left text-sm font-medium transition-colors hover:bg-accent"
              >
                {p.title}
                <span className="text-xs text-muted-foreground">{p.fields.length} fields</span>
              </button>
            ))}
            {providers.length === 0 && !error && (
              <p className="text-sm text-muted-foreground">Loading providers…</p>
            )}
          </div>
        ) : selected.oauth ? (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              You'll be redirected to {selected.title} to authorize access, then
              brought back here. No keys to copy.
            </p>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
        ) : (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="display_name">Display name (optional)</Label>
              <Input
                id="display_name"
                placeholder="e.g. Work Drive"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
            </div>
            {selected.fields.map((field) => (
              <div key={field.key} className="space-y-2">
                <Label htmlFor={field.key}>
                  {field.label}
                  {field.required && <span className="text-destructive"> *</span>}
                </Label>
                <Input
                  id={field.key}
                  type={field.type === "password" ? "password" : "text"}
                  value={values[field.key] ?? ""}
                  onChange={(e) => setValues({ ...values, [field.key]: e.target.value })}
                />
                {field.help && <p className="text-xs text-muted-foreground">{field.help}</p>}
              </div>
            ))}
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
        )}

        <DialogFooter>
          {selected && (
            <>
              <Button variant="ghost" onClick={reset} disabled={busy}>
                Back
              </Button>
              {selected.oauth ? (
                <Button onClick={connect} disabled={busy}>
                  {busy ? "Redirecting…" : `Connect ${selected.title}`}
                </Button>
              ) : (
                <Button onClick={submit} disabled={busy}>
                  {busy ? "Verifying…" : "Verify & add"}
                </Button>
              )}
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
