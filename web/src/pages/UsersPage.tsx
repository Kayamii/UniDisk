import { useCallback, useEffect, useState } from "react";
import { UserPlus, Trash2, KeyRound } from "lucide-react";
import { toast } from "sonner";
import { api, ApiError, type Role, type User } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useConfirm } from "@/components/ConfirmDialog";
import { usePrompt } from "@/components/PromptDialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader,
  DialogTitle, DialogTrigger,
} from "@/components/ui/dialog";

export function UsersPage() {
  const { user: me } = useAuth();
  const confirm = useConfirm();
  const prompt = usePrompt();
  const [users, setUsers] = useState<User[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    Promise.all([api.users(), api.roles()])
      .then(([u, r]) => {
        setUsers(u);
        setRoles(r);
      })
      .catch((e) => setError(String(e.message ?? e)));
  }, []);

  useEffect(load, [load]);

  async function changeRole(id: number, roleId: number) {
    try {
      await api.setUserRole(id, roleId);
      toast.success("Role updated");
      load();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : "Could not update role");
    }
  }

  async function resetPassword(u: User) {
    const pw = await prompt({
      title: `Reset password for ${u.email}`,
      description: "They'll be asked to change it at next login.",
      label: "Temporary password",
      placeholder: "At least 8 characters",
      inputType: "password",
      confirmLabel: "Reset password",
    });
    if (!pw) return;
    try {
      await api.resetUserPassword(u.id, pw);
      toast.success(`Password reset for ${u.email}`, {
        description: "They must change it at next login.",
      });
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : "Reset failed");
    }
  }

  async function remove(u: User) {
    const ok = await confirm({
      title: `Delete ${u.email}?`,
      description: "This permanently removes the user account.",
      confirmLabel: "Delete user",
      destructive: true,
    });
    if (!ok) return;
    try {
      await api.deleteUser(u.id);
      toast.success("User deleted");
      load();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : "Delete failed");
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Users</h1>
          <p className="text-sm text-muted-foreground">
            Create users, assign roles, and reset passwords.
          </p>
        </div>
        <CreateUserDialog roles={roles} onCreated={load} />
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Email</TableHead>
                <TableHead className="w-48">Role</TableHead>
                <TableHead className="w-40">Status</TableHead>
                <TableHead className="w-24" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((u) => (
                <TableRow key={u.id}>
                  <TableCell className="font-medium">
                    {u.email}
                    {u.id === me?.id && (
                      <span className="ml-2 text-xs text-muted-foreground">(you)</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <Select
                      value={u.role_id ? String(u.role_id) : undefined}
                      onValueChange={(v) => changeRole(u.id, Number(v))}
                    >
                      <SelectTrigger className="h-8">
                        <SelectValue placeholder="No role" />
                      </SelectTrigger>
                      <SelectContent>
                        {roles.map((r) => (
                          <SelectItem key={r.id} value={String(r.id)}>
                            {r.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {u.must_change_password ? "Must change password" : "Active"}
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="icon" className="h-8 w-8"
                        title="Reset password" onClick={() => resetPassword(u)}>
                        <KeyRound className="h-4 w-4" />
                      </Button>
                      {u.id !== me?.id && (
                        <Button variant="ghost" size="icon" className="h-8 w-8"
                          title="Delete user" onClick={() => remove(u)}>
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}

function CreateUserDialog({ roles, onCreated }: { roles: Role[]; onCreated: () => void }) {
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [roleId, setRoleId] = useState<string | undefined>();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit() {
    setError(null);
    if (!roleId) {
      setError("Select a role");
      return;
    }
    setBusy(true);
    try {
      await api.createUser(email, password, Number(roleId));
      toast.success(`Created ${email}`);
      setOpen(false);
      setEmail("");
      setPassword("");
      setRoleId(undefined);
      onCreated();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not create user");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <UserPlus className="h-4 w-4" />
          New user
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create user</DialogTitle>
          <DialogDescription>
            They'll sign in with this temporary password and be asked to set their own.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input id="email" type="email" value={email}
              onChange={(e) => setEmail(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="pw">Temporary password</Label>
            <Input id="pw" type="text" value={password} minLength={8}
              placeholder="At least 8 characters"
              onChange={(e) => setPassword(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>Role</Label>
            <Select value={roleId} onValueChange={setRoleId}>
              <SelectTrigger>
                <SelectValue placeholder="Select a role" />
              </SelectTrigger>
              <SelectContent>
                {roles.map((r) => (
                  <SelectItem key={r.id} value={String(r.id)}>
                    {r.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>
        <DialogFooter>
          <Button onClick={submit} disabled={busy}>
            {busy ? "Creating…" : "Create user"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
