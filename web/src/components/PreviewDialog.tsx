import { useEffect, useState } from "react";
import { Download, FileQuestion, Loader2 } from "lucide-react";
import { api, type FileNode } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";

type Kind = "image" | "pdf" | "video" | "audio" | "text" | "none";

function kindFor(name: string, type: string): Kind {
  const t = type.toLowerCase();
  if (t.startsWith("image/")) return "image";
  if (t === "application/pdf") return "pdf";
  if (t.startsWith("video/")) return "video";
  if (t.startsWith("audio/")) return "audio";
  if (
    t.startsWith("text/") ||
    t === "application/json" ||
    t === "application/xml" ||
    /\.(txt|md|csv|log|json|xml|ya?ml|js|ts|tsx|jsx|go|py|css|html|sh|toml|ini)$/i.test(name)
  ) {
    return "text";
  }
  return "none";
}

/**
 * PreviewDialog renders an in-app preview of a file by fetching it inline
 * (authenticated) and choosing a viewer by MIME type. Falls back to a download
 * prompt for types we can't render.
 */
export function PreviewDialog({
  file, open, onOpenChange,
}: {
  file: FileNode | null;
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const [loading, setLoading] = useState(false);
  const [blobUrl, setBlobUrl] = useState<string | null>(null);
  const [kind, setKind] = useState<Kind>("none");
  const [text, setText] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open || !file) return;
    let revoked: string | null = null;
    setLoading(true);
    setError(null);
    setText(null);
    setBlobUrl(null);

    api
      .previewBlob(file.id)
      .then(async ({ url, type }) => {
        revoked = url;
        const k = kindFor(file.name, type);
        setKind(k);
        setBlobUrl(url);
        if (k === "text") {
          const res = await fetch(url);
          const body = await res.text();
          // Guard against huge text files freezing the UI.
          setText(body.length > 500_000 ? body.slice(0, 500_000) + "\n\n…(truncated)" : body);
        }
      })
      .catch((e) => setError(e?.message ?? "Could not load preview"))
      .finally(() => setLoading(false));

    return () => {
      if (revoked) URL.revokeObjectURL(revoked);
    };
  }, [open, file]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle className="truncate pr-8">{file?.name}</DialogTitle>
        </DialogHeader>

        <div className="flex max-h-[70vh] min-h-[16rem] items-center justify-center overflow-auto rounded-md border bg-muted/20">
          {loading ? (
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          ) : error ? (
            <p className="p-6 text-sm text-destructive">{error}</p>
          ) : blobUrl && kind === "image" ? (
            <img src={blobUrl} alt={file?.name} className="max-h-[70vh] max-w-full object-contain" />
          ) : blobUrl && kind === "pdf" ? (
            <iframe src={blobUrl} title={file?.name} className="h-[70vh] w-full" />
          ) : blobUrl && kind === "video" ? (
            <video src={blobUrl} controls className="max-h-[70vh] max-w-full" />
          ) : blobUrl && kind === "audio" ? (
            <div className="p-8">
              <audio src={blobUrl} controls className="w-80" />
            </div>
          ) : kind === "text" && text !== null ? (
            <pre className="w-full overflow-auto p-4 text-left text-xs leading-relaxed">
              <code>{text}</code>
            </pre>
          ) : (
            <div className="flex flex-col items-center gap-3 p-8 text-center">
              <FileQuestion className="h-8 w-8 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                This file type can't be previewed.
              </p>
              {file && (
                <Button size="sm" variant="outline" onClick={() => api.download(file.id, file.name)}>
                  <Download className="h-4 w-4" />
                  Download instead
                </Button>
              )}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
