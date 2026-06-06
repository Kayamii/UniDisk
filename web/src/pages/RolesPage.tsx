import { useCallback, useEffect, useState } from "react";
import { ShieldPlus, Trash2, Lock } from "lucide-react";
import { toast } from "sonner";
import { api, ApiError, type Privilege, type Role } from "@/lib/api";
import { useConfirm } from "@/components/ConfirmDialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader,
  DialogTitle, DialogTrigger,
} from "@/components/ui/dialog";

// Human labels for each privilege key.
const PRIV_LABELS: Record<Privilege, string> = {
  "files.view": "View files",
  "files.upload": "Upload files & create folders",
  "files.download": "Download files",
  "files.delete": "Delete files",
  "providers.manage": "Manage storage providers",
  "users.manage": "Manage users",
  "roles.manage": "Manage roles",
  "settings.manage": "Manage routing settings",
};

export function RolesPage() {
  const confirm = useConfirm();
  const [roles, setRoles] = useState<Role[]>([]);
  const [allPrivs, setAllPrivs] = useState<Privilege[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    Promise.all([api.roles(), api.allPrivileges()])
      .then(([r, p]) => {
        setRoles(r);
        setAllPrivs(p);
      })
      .catch((e) => setError(String(e.message ?? e)));
  }, []);

  useEffect(load, [load]);

  async function remove(r: Role) {
    const ok = await confirm({
      title: `Delete role "${r.name}"?`,
      description: "Users must not be assigned to this role.",
      confirmLabel: "Delete role",
      destructive: true,
    });
    if (!ok) return;
    try {
      await api.deleteRole(r.id);
      toast.success("Role deleted");
      load();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : "Delete failed");
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Roles</h1>
          <p className="text-sm text-muted-foreground">
            Define what users can do. Built-in roles can't be edited.
          </p>
        </div>
        <RoleDialog allPrivs={allPrivs} onSaved={load} />
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="grid gap-3 sm:grid-cols-2">
        {roles.map((r) => (
          <Card key={r.id}>
            <CardHeader className="flex-row items-center justify-between space-y-0 pb-3">
              <CardTitle className="flex items-center gap-2 text-base">
                {r.name}
                {r.is_system && <Lock className="h-3.5 w-3.5 text-muted-foreground" />}
              </CardTitle>
              <div className="flex gap-1">
                {!r.is_system && (
                  <>
                    <RoleDialog allPrivs={allPrivs} existing={r} onSaved={load} />
                    <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => remove(r)}>
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </>
                )}
              </div>
            </CardHeader>
            <CardContent>
              {r.privileges.length === 0 ? (
                <p className="text-sm text-muted-foreground">No privileges.</p>
              ) : (
                <ul className="space-y-1 text-sm text-muted-foreground">
                  {r.privileges.map((p) => (
                    <li key={p}>{PRIV_LABELS[p] ?? p}</li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}

// RoleDialog creates a new role or edits an existing custom role's privileges.
function RoleDialog({
  allPrivs,
  existing,
  onSaved,
}: {
  allPrivs: Privilege[];
  existing?: Role;
  onSaved: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState(existing?.name ?? "");
  const [selected, setSelected] = useState<Set<Privilege>>(new Set(existing?.privileges ?? []));
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function toggle(p: Privilege) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
  }

  async function submit() {
    setError(null);
    setBusy(true);
    try {
      const privs = Array.from(selected);
      if (existing) await api.updateRole(existing.id, privs);
      else await api.createRole(name, privs);
      toast.success(existing ? "Role updated" : "Role created");
      setOpen(false);
      onSaved();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not save role");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { setOpen(o); if (o && existing) setSelected(new Set(existing.privileges)); }}>
      <DialogTrigger asChild>
        {existing ? (
          <Button variant="ghost" size="sm">Edit</Button>
        ) : (
          <Button><ShieldPlus className="h-4 w-4" />New role</Button>
        )}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{existing ? `Edit ${existing.name}` : "Create role"}</DialogTitle>
          <DialogDescription>Choose the privileges this role grants.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {!existing && (
            <div className="space-y-2">
              <Label htmlFor="role-name">Role name</Label>
              <Input id="role-name" value={name} onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Editor" />
            </div>
          )}
          <div className="space-y-2">
            <Label>Privileges</Label>
            <div className="grid gap-2">
              {allPrivs.map((p) => (
                <label key={p} className="flex cursor-pointer items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    className="h-4 w-4 rounded border-input accent-primary"
                    checked={selected.has(p)}
                    onChange={() => toggle(p)}
                  />
                  {PRIV_LABELS[p] ?? p}
                </label>
              ))}
            </div>
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>
        <DialogFooter>
          <Button onClick={submit} disabled={busy}>
            {busy ? "Saving…" : existing ? "Save changes" : "Create role"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
