import { useCallback, useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { HardDrive, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { api, ApiError, type Account } from "@/lib/api";
import { formatBytes } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { AddProviderDialog } from "@/components/AddProviderDialog";
import { useConfirm } from "@/components/ConfirmDialog";

export function ProvidersPage() {
  const confirm = useConfirm();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [params, setParams] = useSearchParams();

  const load = useCallback(() => {
    api.accounts().then(setAccounts).catch((e) => setError(String(e.message ?? e)));
  }, []);

  useEffect(load, [load]);

  // Surface the OAuth callback result (left in the URL) as a toast, then clear it.
  const connected = params.get("connected");
  const oauthError = params.get("error");
  useEffect(() => {
    if (connected) toast.success(`Connected ${connected}`);
    if (oauthError) toast.error(`Connection failed: ${oauthError.replace(/_/g, " ")}`);
    if (connected || oauthError) setParams({}, { replace: true });
  }, [connected, oauthError, setParams]);

  async function remove(id: number) {
    const ok = await confirm({
      title: "Disconnect this account?",
      description: "Files stored on it will become unavailable in the pool.",
      confirmLabel: "Disconnect",
      destructive: true,
    });
    if (!ok) return;
    try {
      await api.deleteAccount(id);
      toast.success("Account disconnected");
      load();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : "Could not disconnect");
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Providers</h1>
          <p className="text-sm text-muted-foreground">
            Connected storage accounts that make up your pool.
          </p>
        </div>
        <AddProviderDialog onAdded={load} />
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {accounts.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
            <HardDrive className="h-8 w-8 text-muted-foreground" />
            <p className="text-sm font-medium">No providers connected</p>
            <p className="text-sm text-muted-foreground">
              Add a provider to start building your storage pool.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3">
          {accounts.map((a) => (
            <Card key={a.id}>
              <CardContent className="flex items-center justify-between p-4">
                <div className="flex items-center gap-3">
                  <div className="flex h-9 w-9 items-center justify-center rounded-md bg-secondary">
                    <HardDrive className="h-4 w-4" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">{a.display_name}</p>
                    <p className="text-xs text-muted-foreground">
                      {a.provider} · {formatBytes(a.used_bytes)} of{" "}
                      {a.quota_bytes > 0 ? formatBytes(a.quota_bytes) : "unlimited"} ·{" "}
                      <span className={a.status === "active" ? "text-foreground" : "text-destructive"}>
                        {a.status}
                      </span>
                    </p>
                  </div>
                </div>
                <Button variant="ghost" size="icon" onClick={() => remove(a.id)} title="Disconnect">
                  <Trash2 className="h-4 w-4" />
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
