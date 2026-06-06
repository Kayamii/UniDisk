import { useCallback, useEffect, useState } from "react";
import { KeyRound, Plus, Trash2, Copy, Check } from "lucide-react";
import { toast } from "sonner";
import { api, ApiError, type APIKey, type Privilege } from "@/lib/api";
import { useConfirm } from "@/components/ConfirmDialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader,
  DialogTitle, DialogTrigger,
} from "@/components/ui/dialog";

const PRIV_LABELS: Partial<Record<Privilege, string>> = {
  "files.view": "List & view files",
  "files.upload": "Upload files & folders",
  "files.download": "Download files",
  "files.delete": "Delete files",
};

export function ApiKeysPage() {
  const confirm = useConfirm();
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    api.apiKeys().then(setKeys).catch((e) => setError(String(e.message ?? e)));
  }, []);
  useEffect(load, [load]);

  async function revoke(k: APIKey) {
    const ok = await confirm({
      title: `Revoke "${k.name}"?`,
      description: "Any app using this key will immediately lose access.",
      confirmLabel: "Revoke key",
      destructive: true,
    });
    if (!ok) return;
    try {
      await api.deleteApiKey(k.id);
      toast.success("Key revoked");
      load();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : "Revoke failed");
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">API Keys</h1>
          <p className="text-sm text-muted-foreground">
            Generate keys to use UniDisk as storage from your apps and scripts.
          </p>
        </div>
        <CreateKeyDialog onCreated={load} />
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {keys.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
            <KeyRound className="h-8 w-8 text-muted-foreground" />
            <p className="text-sm font-medium">No API keys yet</p>
            <p className="text-sm text-muted-foreground">
              Create one to upload or download files programmatically.
            </p>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Key</TableHead>
                  <TableHead>Permissions</TableHead>
                  <TableHead className="w-36">Expires</TableHead>
                  <TableHead className="w-12" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {keys.map((k) => (
                  <TableRow key={k.id}>
                    <TableCell className="font-medium">{k.name}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {k.key_prefix}…
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {k.privileges.map((p) => p.replace("files.", "")).join(", ")}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {k.expires_at ? new Date(k.expires_at).toLocaleDateString() : "Never"}
                    </TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => revoke(k)}>
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function CreateKeyDialog({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false);
  const [grantable, setGrantable] = useState<Privilege[]>([]);
  const [name, setName] = useState("");
  const [selected, setSelected] = useState<Set<Privilege>>(new Set());
  const [expiresDays, setExpiresDays] = useState("0");
  const [created, setCreated] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (open && grantable.length === 0) {
      api.grantableKeyPrivileges().then(setGrantable).catch(() => {});
    }
  }, [open, grantable.length]);

  function reset() {
    setName("");
    setSelected(new Set());
    setExpiresDays("0");
    setCreated(null);
    setCopied(false);
    setError(null);
  }

  function toggle(p: Privilege) {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(p) ? next.delete(p) : next.add(p);
      return next;
    });
  }

  async function submit() {
    setError(null);
    if (!name.trim()) { setError("Give the key a name"); return; }
    if (selected.size === 0) { setError("Select at least one permission"); return; }
    setBusy(true);
    try {
      const res = await api.createApiKey(name.trim(), Array.from(selected), Number(expiresDays) || 0);
      setCreated(res.key);
      onCreated();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not create key");
    } finally {
      setBusy(false);
    }
  }

  async function copy() {
    if (!created) return;
    await navigator.clipboard.writeText(created);
    setCopied(true);
    toast.success("Key copied");
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { setOpen(o); if (!o) reset(); }}>
      <DialogTrigger asChild>
        <Button><Plus className="h-4 w-4" />New API key</Button>
      </DialogTrigger>
      <DialogContent>
        {created ? (
          <>
            <DialogHeader>
              <DialogTitle>Copy your API key</DialogTitle>
              <DialogDescription>
                This is shown only once. Store it securely — you won't be able to see it again.
              </DialogDescription>
            </DialogHeader>
            <div className="flex items-center gap-2 rounded-md border bg-muted/40 p-3">
              <code className="flex-1 break-all font-mono text-sm">{created}</code>
              <Button size="icon" variant="ghost" className="h-8 w-8 shrink-0" onClick={copy}>
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
              </Button>
            </div>
            <DialogFooter>
              <Button onClick={() => setOpen(false)}>Done</Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Create API key</DialogTitle>
              <DialogDescription>
                The key can only use permissions you hold. It can never manage providers, users, or roles.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="key-name">Name</Label>
                <Input id="key-name" placeholder="e.g. Website uploads"
                  value={name} onChange={(e) => setName(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Permissions</Label>
                {grantable.length === 0 ? (
                  <p className="text-sm text-muted-foreground">You have no file permissions to grant.</p>
                ) : (
                  <div className="grid gap-2">
                    {grantable.map((p) => (
                      <label key={p} className="flex cursor-pointer items-center gap-2 text-sm">
                        <input type="checkbox" className="h-4 w-4 rounded border-input accent-primary"
                          checked={selected.has(p)} onChange={() => toggle(p)} />
                        {PRIV_LABELS[p] ?? p}
                      </label>
                    ))}
                  </div>
                )}
              </div>
              <div className="space-y-2">
                <Label htmlFor="expiry">Expires in (days)</Label>
                <Input id="expiry" type="number" min={0} className="w-32"
                  value={expiresDays} onChange={(e) => setExpiresDays(e.target.value)} />
                <p className="text-xs text-muted-foreground">0 = never expires.</p>
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
            </div>
            <DialogFooter>
              <Button onClick={submit} disabled={busy}>
                {busy ? "Creating…" : "Create key"}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
