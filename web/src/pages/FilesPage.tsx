import { useCallback, useEffect, useRef, useState } from "react";
import {
  Upload, FolderPlus, MoreVertical, Download, Pencil, Trash2, ChevronRight,
  Loader2, Search, LayoutGrid, List, FolderOpen, X, Link2, Eye,
} from "lucide-react";
import { toast } from "sonner";
import { api, ApiError, type FileNode } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useConfirm } from "@/components/ConfirmDialog";
import { usePrompt } from "@/components/PromptDialog";
import { ShareDialog } from "@/components/ShareDialog";
import { PreviewDialog } from "@/components/PreviewDialog";
import { fileIcon } from "@/lib/fileIcon";
import { formatBytes, cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

interface Crumb {
  id: number | null;
  name: string;
}
interface UploadState {
  name: string;
  loaded: number;
  total: number;
  bps: number;
}
type ViewMode = "list" | "grid";

export function FilesPage() {
  const { can } = useAuth();
  const confirm = useConfirm();
  const prompt = usePrompt();
  const [path, setPath] = useState<Crumb[]>([{ id: null, name: "Home" }]);
  const [items, setItems] = useState<FileNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [uploads, setUploads] = useState<UploadState[]>([]);
  const [downloading, setDownloading] = useState<Record<number, string>>({});
  const [view, setView] = useState<ViewMode>(
    () => (localStorage.getItem("unidisk_view") as ViewMode) || "list"
  );
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<FileNode[] | null>(null);
  const [dragging, setDragging] = useState(false);
  const [shareFile, setShareFile] = useState<FileNode | null>(null);
  const [previewFile, setPreviewFile] = useState<FileNode | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const dragDepth = useRef(0);

  const current = path[path.length - 1];

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    api
      .files(current.id)
      .then(setItems)
      .catch((e) => setError(e instanceof ApiError ? e.message : String(e)))
      .finally(() => setLoading(false));
  }, [current.id]);

  useEffect(load, [load]);

  // Debounced search across the whole pool.
  useEffect(() => {
    const q = query.trim();
    if (!q) {
      setResults(null);
      return;
    }
    const t = setTimeout(() => {
      api.searchFiles(q).then(setResults).catch(() => setResults([]));
    }, 250);
    return () => clearTimeout(t);
  }, [query]);

  function setViewMode(v: ViewMode) {
    setView(v);
    localStorage.setItem("unidisk_view", v);
  }

  function openFolder(node: FileNode) {
    setPath([...path, { id: node.id, name: node.name }]);
  }
  function goTo(index: number) {
    setPath(path.slice(0, index + 1));
  }

  async function uploadFiles(files: File[]) {
    setError(null);
    try {
      for (const f of files) {
        setUploads((u) => [...u, { name: f.name, loaded: 0, total: f.size, bps: 0 }]);
        await api.upload(f, current.id, ({ loaded, total, bps }) => {
          setUploads((u) => u.map((it) => (it.name === f.name ? { ...it, loaded, total, bps } : it)));
        });
        setUploads((u) => u.filter((it) => it.name !== f.name));
        toast.success(`Uploaded ${f.name}`);
      }
      load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Upload failed");
      setUploads([]);
    }
  }

  async function onPick(e: React.ChangeEvent<HTMLInputElement>) {
    const files = e.target.files;
    if (files && files.length) await uploadFiles(Array.from(files));
    if (fileInput.current) fileInput.current.value = "";
  }

  // Drag-and-drop handlers (depth counter avoids flicker over child elements).
  function onDragEnter(e: React.DragEvent) {
    if (!canUpload) return;
    e.preventDefault();
    dragDepth.current += 1;
    if (e.dataTransfer.types.includes("Files")) setDragging(true);
  }
  function onDragLeave(e: React.DragEvent) {
    e.preventDefault();
    dragDepth.current -= 1;
    if (dragDepth.current <= 0) setDragging(false);
  }
  function onDrop(e: React.DragEvent) {
    e.preventDefault();
    dragDepth.current = 0;
    setDragging(false);
    if (!canUpload) return;
    const files = Array.from(e.dataTransfer.files);
    if (files.length) uploadFiles(files);
  }

  async function download(node: FileNode) {
    setDownloading((d) => ({ ...d, [node.id]: "Preparing…" }));
    try {
      await api.download(node.id, node.name, ({ loaded, total }) => {
        const label = total > 0 ? `${Math.round((loaded / total) * 100)}%` : formatBytes(loaded);
        setDownloading((d) => ({ ...d, [node.id]: label }));
      });
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : "Download failed");
    } finally {
      setDownloading((d) => {
        const next = { ...d };
        delete next[node.id];
        return next;
      });
    }
  }

  async function newFolder() {
    const name = await prompt({
      title: "New folder",
      label: "Folder name",
      placeholder: "e.g. Documents",
      confirmLabel: "Create",
    });
    if (!name) return;
    try {
      await api.createFolder(name, current.id);
      toast.success("Folder created");
      load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Could not create folder");
    }
  }

  async function rename(node: FileNode) {
    const name = await prompt({
      title: `Rename ${node.is_dir ? "folder" : "file"}`,
      label: "New name",
      defaultValue: node.name,
      confirmLabel: "Rename",
    });
    if (!name || name === node.name) return;
    try {
      await api.renameFile(node.id, name);
      toast.success("Renamed");
      load();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : "Rename failed");
    }
  }

  async function remove(node: FileNode) {
    const ok = await confirm({
      title: `Delete "${node.name}"?`,
      description: node.is_dir
        ? "This deletes the folder and everything inside it."
        : "This removes the file from its provider.",
      confirmLabel: "Delete",
      destructive: true,
    });
    if (!ok) return;
    try {
      await api.deleteFile(node.id);
      toast.success("Deleted");
      load();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : "Delete failed");
    }
  }

  const canUpload = can("files.upload");
  const canDownload = can("files.download");
  const canDelete = can("files.delete");
  const uploading = uploads.length > 0;
  const searching = results !== null;
  const shown = searching ? results! : items;

  const rowActions = (node: FileNode) => {
    const dl = downloading[node.id];
    if (dl) {
      return (
        <div className="flex items-center justify-end gap-1.5 text-xs text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          {dl}
        </div>
      );
    }
    return (
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" className="h-7 w-7">
            <MoreVertical className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {!node.is_dir && canDownload && (
            <DropdownMenuItem onSelect={() => setPreviewFile(node)}>
              <Eye className="h-4 w-4" />
              View
            </DropdownMenuItem>
          )}
          {!node.is_dir && canDownload && (
            <DropdownMenuItem onSelect={() => download(node)}>
              <Download className="h-4 w-4" />
              Download
            </DropdownMenuItem>
          )}
          {!node.is_dir && canDownload && (
            <DropdownMenuItem onSelect={() => setShareFile(node)}>
              <Link2 className="h-4 w-4" />
              Share link
            </DropdownMenuItem>
          )}
          {canUpload && (
            <DropdownMenuItem onSelect={() => rename(node)}>
              <Pencil className="h-4 w-4" />
              Rename
            </DropdownMenuItem>
          )}
          {canDelete && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="text-destructive focus:text-destructive"
                onSelect={() => remove(node)}
              >
                <Trash2 className="h-4 w-4" />
                Delete
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    );
  };

  return (
    <div
      className="relative space-y-5"
      onDragEnter={onDragEnter}
      onDragOver={(e) => canUpload && e.preventDefault()}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
    >
      {/* Drop overlay */}
      {dragging && (
        <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center rounded-xl border-2 border-dashed border-primary bg-background/80 backdrop-blur-sm">
          <div className="flex flex-col items-center gap-2 text-primary">
            <Upload className="h-8 w-8" />
            <p className="text-sm font-medium">Drop files to upload</p>
          </div>
        </div>
      )}

      {/* Toolbar */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-1 text-sm">
          {searching ? (
            <span className="text-muted-foreground">
              Search results for “{query.trim()}”
            </span>
          ) : (
            path.map((c, i) => (
              <span key={i} className="flex items-center gap-1">
                {i > 0 && <ChevronRight className="h-4 w-4 text-muted-foreground" />}
                <button
                  onClick={() => goTo(i)}
                  className={
                    i === path.length - 1
                      ? "font-medium"
                      : "text-muted-foreground hover:text-foreground"
                  }
                >
                  {c.name}
                </button>
              </span>
            ))
          )}
        </div>

        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Search files…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="h-9 w-44 pl-8 pr-8"
            />
            {query && (
              <button
                onClick={() => setQuery("")}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            )}
          </div>

          <div className="flex rounded-md border">
            <Button
              variant={view === "list" ? "secondary" : "ghost"}
              size="icon" className="h-9 w-9 rounded-r-none"
              onClick={() => setViewMode("list")}
            >
              <List className="h-4 w-4" />
            </Button>
            <Button
              variant={view === "grid" ? "secondary" : "ghost"}
              size="icon" className="h-9 w-9 rounded-l-none"
              onClick={() => setViewMode("grid")}
            >
              <LayoutGrid className="h-4 w-4" />
            </Button>
          </div>

          {canUpload && !searching && (
            <>
              <Button variant="outline" size="sm" onClick={newFolder}>
                <FolderPlus className="h-4 w-4" />
                New folder
              </Button>
              <Button size="sm" disabled={uploading} onClick={() => fileInput.current?.click()}>
                <Upload className="h-4 w-4" />
                {uploading ? "Uploading…" : "Upload"}
              </Button>
              <input ref={fileInput} type="file" multiple className="hidden" onChange={onPick} />
            </>
          )}
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* Active uploads */}
      {uploads.map((u) => {
        const pct = u.total > 0 ? (u.loaded / u.total) * 100 : 0;
        return (
          <Card key={u.name}>
            <CardContent className="space-y-2 p-4">
              <div className="flex items-center justify-between text-sm">
                <span className="truncate font-medium">{u.name}</span>
                <span className="shrink-0 text-muted-foreground">
                  {Math.round(pct)}% · {formatBytes(u.bps)}/s
                </span>
              </div>
              <Progress value={pct} />
              <p className="text-xs text-muted-foreground">
                {formatBytes(u.loaded)} of {formatBytes(u.total)}
              </p>
            </CardContent>
          </Card>
        );
      })}

      {/* Content */}
      {loading && !searching ? (
        <LoadingState view={view} />
      ) : shown.length === 0 ? (
        <EmptyState searching={searching} canUpload={canUpload && !searching} onUpload={() => fileInput.current?.click()} />
      ) : view === "grid" ? (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {shown.map((node) => {
            const { Icon, tint } = fileIcon(node);
            return (
              <div
                key={node.id}
                className="group relative flex flex-col items-center gap-2 rounded-lg border p-4 text-center transition-colors hover:bg-accent/50"
              >
                <button
                  className="flex flex-col items-center gap-2"
                  onClick={() => (node.is_dir ? openFolder(node) : canDownload && setPreviewFile(node))}
                >
                  <Icon className={cn("h-10 w-10", tint)} />
                  <span className="line-clamp-2 break-all text-sm font-medium">{node.name}</span>
                  <span className="text-xs text-muted-foreground">
                    {node.is_dir ? "Folder" : formatBytes(node.size_bytes)}
                  </span>
                </button>
                <div className="absolute right-1 top-1 opacity-0 transition-opacity group-hover:opacity-100">
                  {rowActions(node)}
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead className="w-32">Size</TableHead>
                <TableHead className="w-44">Modified</TableHead>
                <TableHead className="w-12" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {shown.map((node) => {
                const { Icon, tint } = fileIcon(node);
                return (
                  <TableRow key={node.id}>
                    <TableCell>
                      <button
                        className="flex items-center gap-2.5 text-left font-medium hover:underline"
                        onClick={() => (node.is_dir ? openFolder(node) : canDownload && setPreviewFile(node))}
                      >
                        <Icon className={cn("h-4 w-4", tint)} />
                        {node.name}
                      </button>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {node.is_dir ? "—" : formatBytes(node.size_bytes)}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {new Date(node.updated_at).toLocaleString()}
                    </TableCell>
                    <TableCell>{rowActions(node)}</TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      )}

      <ShareDialog
        file={shareFile}
        open={shareFile !== null}
        onOpenChange={(o) => { if (!o) setShareFile(null); }}
      />
      <PreviewDialog
        file={previewFile}
        open={previewFile !== null}
        onOpenChange={(o) => { if (!o) setPreviewFile(null); }}
      />
    </div>
  );
}

function LoadingState({ view }: { view: ViewMode }) {
  if (view === "grid") {
    return (
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {Array.from({ length: 8 }).map((_, i) => (
          <Skeleton key={i} className="h-28 w-full" />
        ))}
      </div>
    );
  }
  return (
    <div className="space-y-2 rounded-lg border p-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-9 w-full" />
      ))}
    </div>
  );
}

function EmptyState({
  searching, canUpload, onUpload,
}: { searching: boolean; canUpload: boolean; onUpload: () => void }) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed py-16 text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
        {searching ? <Search className="h-6 w-6 text-muted-foreground" /> : <FolderOpen className="h-6 w-6 text-muted-foreground" />}
      </div>
      <div>
        <p className="text-sm font-medium">{searching ? "No matching files" : "This folder is empty"}</p>
        <p className="text-sm text-muted-foreground">
          {searching ? "Try a different search term." : canUpload ? "Drag files here or use the upload button." : "Nothing here yet."}
        </p>
      </div>
      {canUpload && !searching && (
        <Button size="sm" variant="outline" onClick={onUpload}>
          <Upload className="h-4 w-4" />
          Upload files
        </Button>
      )}
    </div>
  );
}
