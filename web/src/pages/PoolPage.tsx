import { useEffect, useState } from "react";
import { HardDrive } from "lucide-react";
import { api, type PoolStats } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { formatBytes } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";

export function PoolPage() {
  const { can } = useAuth();
  const [stats, setStats] = useState<PoolStats | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.stats().then(setStats).catch((e) => setError(String(e.message ?? e)));
  }, []);

  if (error) return <p className="text-sm text-destructive">{error}</p>;
  if (!stats) return <p className="text-sm text-muted-foreground">Loading…</p>;

  const usedPct = stats.total_bytes > 0 ? (stats.used_bytes / stats.total_bytes) * 100 : 0;

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Storage Pool</h1>
        <p className="text-sm text-muted-foreground">
          Aggregated capacity across {stats.account_count}{" "}
          {stats.account_count === 1 ? "account" : "accounts"}.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Total capacity</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <Progress value={usedPct} />
          <div className="grid grid-cols-3 gap-4 text-center">
            <Metric label="Used" value={formatBytes(stats.used_bytes)} />
            <Metric label="Available" value={formatBytes(stats.available_bytes)} />
            <Metric label="Total" value={formatBytes(stats.total_bytes)} />
          </div>
        </CardContent>
      </Card>

      {can("settings.manage") && <RoutingCard />}

      <div className="space-y-3">
        <h2 className="text-sm font-medium text-muted-foreground">Accounts</h2>
        {stats.accounts.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No accounts connected yet. Add one from the Providers page.
          </p>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            {stats.accounts.map((a) => {
              const pct = a.quota_bytes > 0 ? (a.used_bytes / a.quota_bytes) * 100 : 0;
              return (
                <Card key={a.id}>
                  <CardContent className="space-y-3 p-4">
                    <div className="flex items-center gap-3">
                      <div className="flex h-8 w-8 items-center justify-center rounded-md bg-secondary">
                        <HardDrive className="h-4 w-4" />
                      </div>
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium">{a.display_name}</p>
                        <p className="text-xs text-muted-foreground">{a.provider}</p>
                      </div>
                    </div>
                    <Progress value={pct} />
                    <p className="text-xs text-muted-foreground">
                      {formatBytes(a.used_bytes)} of{" "}
                      {a.quota_bytes > 0 ? formatBytes(a.quota_bytes) : "unlimited"}
                    </p>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xl font-semibold">{value}</p>
      <p className="text-xs text-muted-foreground">{label}</p>
    </div>
  );
}

// RoutingCard lets the user set the fill threshold that drives upload routing.
function RoutingCard() {
  const [pct, setPct] = useState<number | null>(null);
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.getSettings().then((s) => setPct(s.fill_threshold_pct)).catch(() => setPct(80));
  }, []);

  async function save() {
    if (pct == null) return;
    setBusy(true);
    setSaved(false);
    try {
      const s = await api.updateSettings({ fill_threshold_pct: pct });
      setPct(s.fill_threshold_pct);
      setSaved(true);
      setTimeout(() => setSaved(false), 2500);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Routing</CardTitle>
        <CardDescription>
          New files are spread round-robin across accounts below the fill
          threshold. When every account is above it, the one with the most free
          space is used.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-end gap-3">
          <div className="space-y-1.5">
            <label htmlFor="threshold" className="text-sm font-medium">
              Fill threshold
            </label>
            <div className="flex items-center gap-2">
              <Input
                id="threshold"
                type="number"
                min={1}
                max={100}
                className="w-24"
                value={pct ?? ""}
                onChange={(e) => setPct(e.target.value === "" ? null : Number(e.target.value))}
              />
              <span className="text-sm text-muted-foreground">% used</span>
            </div>
          </div>
          <Button onClick={save} disabled={busy || pct == null}>
            {busy ? "Saving…" : "Save"}
          </Button>
          {saved && <span className="pb-2 text-sm text-muted-foreground">Saved</span>}
        </div>
      </CardContent>
    </Card>
  );
}
