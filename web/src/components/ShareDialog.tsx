import { useState } from "react";
import { Check, Copy, Link2 } from "lucide-react";
import { toast } from "sonner";
import { api, ApiError, type FileNode } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";

/**
 * ShareDialog creates a public presigned download URL for a single file. It's
 * controlled by the parent (open + file), so the file row can trigger it.
 */
export function ShareDialog({
  file, open, onOpenChange,
}: {
  file: FileNode | null;
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const [hours, setHours] = useState("24");
  const [url, setUrl] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function reset() {
    setHours("24");
    setUrl(null);
    setCopied(false);
    setError(null);
  }

  async function create() {
    if (!file) return;
    setError(null);
    setBusy(true);
    try {
      const link = await api.createPresigned(file.id, Number(hours) || 0);
      setUrl(link.url);
      toast.success("Share link created");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not create link");
    } finally {
      setBusy(false);
    }
  }

  async function copy() {
    if (!url) return;
    await navigator.clipboard.writeText(url);
    setCopied(true);
    toast.success("Link copied");
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { onOpenChange(o); if (!o) reset(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Share “{file?.name}”</DialogTitle>
          <DialogDescription>
            Creates a public, direct-download link. Anyone with the link can download the file.
          </DialogDescription>
        </DialogHeader>

        {url ? (
          <div className="space-y-3">
            <div className="flex items-center gap-2 rounded-md border bg-muted/40 p-3">
              <Link2 className="h-4 w-4 shrink-0 text-muted-foreground" />
              <code className="flex-1 break-all text-sm">{url}</code>
              <Button size="icon" variant="ghost" className="h-8 w-8 shrink-0" onClick={copy}>
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              {Number(hours) > 0 ? `Expires in ${hours} hour(s).` : "This link never expires."}
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            <Label htmlFor="hours">Expires in (hours)</Label>
            <Input id="hours" type="number" min={0} className="w-32"
              value={hours} onChange={(e) => setHours(e.target.value)} />
            <p className="text-xs text-muted-foreground">0 = never expires.</p>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
        )}

        <DialogFooter>
          {url ? (
            <Button onClick={() => onOpenChange(false)}>Done</Button>
          ) : (
            <Button onClick={create} disabled={busy}>
              {busy ? "Creating…" : "Create link"}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
