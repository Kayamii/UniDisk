import {
  Folder,
  File,
  FileText,
  FileImage,
  FileVideo,
  FileAudio,
  FileArchive,
  FileCode,
  FileSpreadsheet,
  type LucideIcon,
} from "lucide-react";
import type { FileNode } from "./api";

// Map a lowercased extension to an icon and a subtle tint class.
const byExt: Record<string, { icon: LucideIcon; tint: string }> = {
  // images
  png: { icon: FileImage, tint: "text-emerald-500" },
  jpg: { icon: FileImage, tint: "text-emerald-500" },
  jpeg: { icon: FileImage, tint: "text-emerald-500" },
  gif: { icon: FileImage, tint: "text-emerald-500" },
  webp: { icon: FileImage, tint: "text-emerald-500" },
  svg: { icon: FileImage, tint: "text-emerald-500" },
  // video
  mp4: { icon: FileVideo, tint: "text-violet-500" },
  mov: { icon: FileVideo, tint: "text-violet-500" },
  mkv: { icon: FileVideo, tint: "text-violet-500" },
  webm: { icon: FileVideo, tint: "text-violet-500" },
  // audio
  mp3: { icon: FileAudio, tint: "text-pink-500" },
  wav: { icon: FileAudio, tint: "text-pink-500" },
  flac: { icon: FileAudio, tint: "text-pink-500" },
  // archives
  zip: { icon: FileArchive, tint: "text-amber-500" },
  rar: { icon: FileArchive, tint: "text-amber-500" },
  "7z": { icon: FileArchive, tint: "text-amber-500" },
  tar: { icon: FileArchive, tint: "text-amber-500" },
  gz: { icon: FileArchive, tint: "text-amber-500" },
  // documents
  pdf: { icon: FileText, tint: "text-red-500" },
  doc: { icon: FileText, tint: "text-blue-500" },
  docx: { icon: FileText, tint: "text-blue-500" },
  txt: { icon: FileText, tint: "text-muted-foreground" },
  md: { icon: FileText, tint: "text-muted-foreground" },
  // spreadsheets
  xls: { icon: FileSpreadsheet, tint: "text-green-600" },
  xlsx: { icon: FileSpreadsheet, tint: "text-green-600" },
  csv: { icon: FileSpreadsheet, tint: "text-green-600" },
  // code
  js: { icon: FileCode, tint: "text-yellow-500" },
  ts: { icon: FileCode, tint: "text-blue-400" },
  tsx: { icon: FileCode, tint: "text-blue-400" },
  jsx: { icon: FileCode, tint: "text-blue-400" },
  go: { icon: FileCode, tint: "text-cyan-500" },
  py: { icon: FileCode, tint: "text-blue-300" },
  json: { icon: FileCode, tint: "text-orange-400" },
  html: { icon: FileCode, tint: "text-orange-500" },
  css: { icon: FileCode, tint: "text-sky-500" },
};

/** fileIcon returns the icon + tint class for a node by its type/extension. */
export function fileIcon(node: Pick<FileNode, "name" | "is_dir">): { Icon: LucideIcon; tint: string } {
  if (node.is_dir) return { Icon: Folder, tint: "text-muted-foreground" };
  const ext = node.name.split(".").pop()?.toLowerCase() ?? "";
  const match = byExt[ext];
  return match ? { Icon: match.icon, tint: match.tint } : { Icon: File, tint: "text-muted-foreground" };
}
