// Seed data for the demo build. Everything here is fictional — it exists only
// to make the public demo look like a real, lived-in UniDisk instance.

import type {
  Account, APIKey, FileNode, PresignedURL, Privilege, Role, Settings, User,
} from "../api";

export const GB = 1024 ** 3;
export const MB = 1024 ** 2;

/** Fixed "now" offsets keep the seeded timestamps plausible on any day. */
function daysAgo(n: number): string {
  return new Date(Date.now() - n * 86_400_000).toISOString();
}

export const ALL_PRIVILEGES: Privilege[] = [
  "files.view", "files.upload", "files.download", "files.delete",
  "providers.manage", "users.manage", "roles.manage", "settings.manage",
];

export const DEMO_ADMIN: User = {
  id: 1,
  email: "admin@unidisk.local",
  role_id: 1,
  role_name: "Admin",
  must_change_password: false,
  privileges: ALL_PRIVILEGES,
  created_at: daysAgo(96),
};

export function seedUsers(): User[] {
  return [
    DEMO_ADMIN,
    {
      id: 2, email: "maya@unidisk.local", role_id: 2, role_name: "Viewer",
      must_change_password: false,
      privileges: ["files.view", "files.download"],
      created_at: daysAgo(41),
    },
    {
      id: 3, email: "devbot@unidisk.local", role_id: 3, role_name: "Uploader",
      must_change_password: false,
      privileges: ["files.view", "files.upload", "files.download"],
      created_at: daysAgo(12),
    },
  ];
}

export function seedRoles(): Role[] {
  return [
    { id: 1, name: "Admin", is_system: true, privileges: ALL_PRIVILEGES },
    { id: 2, name: "Viewer", is_system: true, privileges: ["files.view", "files.download"] },
    {
      id: 3, name: "Uploader", is_system: false,
      privileges: ["files.view", "files.upload", "files.download"],
    },
  ];
}

// A pool built from several free tiers — the pitch of the project, made literal.
export function seedAccounts(): Account[] {
  return [
    {
      id: 1, provider: "googledrive", display_name: "Google Drive · personal",
      status: "active", quota_bytes: 15 * GB, used_bytes: 11.2 * GB,
      priority: 0, created_at: daysAgo(96),
    },
    {
      id: 2, provider: "googledrive", display_name: "Google Drive · archive",
      status: "active", quota_bytes: 15 * GB, used_bytes: 3.4 * GB,
      priority: 0, created_at: daysAgo(88),
    },
    {
      id: 3, provider: "onedrive", display_name: "OneDrive · work",
      status: "active", quota_bytes: 5 * GB, used_bytes: 2.1 * GB,
      priority: 0, created_at: daysAgo(60),
    },
    {
      id: 4, provider: "dropbox", display_name: "Dropbox · shared",
      status: "active", quota_bytes: 2 * GB, used_bytes: 1.7 * GB,
      priority: 0, created_at: daysAgo(45),
    },
    {
      id: 5, provider: "s3", display_name: "Backblaze B2 · cold",
      status: "active", quota_bytes: 50 * GB, used_bytes: 6.9 * GB,
      priority: 0, created_at: daysAgo(21),
    },
  ];
}

export function seedSettings(): Settings {
  return { fill_threshold_pct: 80 };
}

export function seedApiKeys(): APIKey[] {
  return [
    {
      id: 1, name: "backup-cron", key_prefix: "udk_9f3a",
      privileges: ["files.view", "files.upload"],
      created_at: daysAgo(30), expires_at: null, last_used_at: daysAgo(0.2),
    },
    {
      id: 2, name: "website-assets", key_prefix: "udk_c71b",
      privileges: ["files.view", "files.download"],
      created_at: daysAgo(14), expires_at: daysAgo(-76), last_used_at: daysAgo(2),
    },
  ];
}

export function seedPresigned(): PresignedURL[] {
  return [
    {
      id: 1, token: "kR3xN8pQ2vLm", file_id: 21, file_name: "quarterly-report.pdf",
      created_at: daysAgo(3), expires_at: daysAgo(-4), downloads: 12,
      url: `${location.origin}${import.meta.env.BASE_URL}s/kR3xN8pQ2vLm`,
    },
  ];
}

/**
 * A demo file tree. `body` (when present) is inlined text used to make preview
 * and download work for seeded files without any network access; binary-ish
 * files are generated on demand instead (see demoFileBytes).
 */
export interface SeedFile extends FileNode {
  body?: string;
}

export function seedFiles(): SeedFile[] {
  const f = (
    id: number, parent_id: number | null, name: string, is_dir: boolean,
    size_bytes: number, mime_type: string, account_id: number | undefined,
    ageDays: number, body?: string
  ): SeedFile => ({
    id, parent_id, name, is_dir, size_bytes, mime_type, account_id,
    created_at: daysAgo(ageDays), updated_at: daysAgo(ageDays), body,
  });

  return [
    // Root folders
    f(1, null, "Documents", true, 0, "", undefined, 90),
    f(2, null, "Photos", true, 0, "", undefined, 88),
    f(3, null, "Projects", true, 0, "", undefined, 70),
    f(4, null, "Backups", true, 0, "", undefined, 55),

    // Root files
    f(20, null, "README.md", false, 4_812, "text/markdown", 1, 6,
      `# Unified pool\n\nThis folder is served from five different cloud accounts.\nYou can't tell which is which — that's the point.\n\n- Uploads are routed round-robin across accounts under 80% full\n- Metadata lives in UniDisk; the bytes live in your providers\n- Share links are presigned and expire on a schedule you pick\n`),
    f(21, null, "quarterly-report.pdf", false, 2.4 * MB, "application/pdf", 3, 3),
    f(22, null, "budget-2026.csv", false, 18_204, "text/csv", 2, 9,
      "month,plan,actual,delta\nJan,4200,3980,-220\nFeb,4200,4310,110\nMar,4400,4405,5\nApr,4400,4120,-280\nMay,4600,4590,-10\nJun,4600,4880,280\n"),

    // Documents/
    f(30, 1, "Contracts", true, 0, "", undefined, 80),
    f(31, 1, "meeting-notes.md", false, 7_340, "text/markdown", 4, 2,
      `# Weekly sync\n\n## Storage\n- Dropbox account is at 85% — routing skipped it this week\n- Added Backblaze B2 for cold archives (50 GB)\n\n## Next\n- [ ] Rotate the backup-cron API key\n- [ ] Move 2023 photos to the archive account\n`),
    f(32, 1, "invoice-0042.pdf", false, 184_320, "application/pdf", 1, 17),
    f(33, 1, "onboarding.txt", false, 2_048, "text/plain", 2, 24,
      "Welcome!\n\n1. Sign in with the credentials your admin gave you.\n2. Change your password when prompted.\n3. Drag files anywhere onto the file list to upload.\n\nYour files are spread across every connected account automatically.\n"),

    // Documents/Contracts/
    f(40, 30, "msa-signed.pdf", false, 512_000, "application/pdf", 5, 63),
    f(41, 30, "nda-template.md", false, 3_100, "text/markdown", 2, 71,
      "# Mutual NDA (template)\n\nThis is placeholder text in a demo instance.\nNothing here is a real agreement.\n"),

    // Photos/
    f(50, 2, "iceland-2025.jpg", false, 4.1 * MB, "image/jpeg", 1, 34),
    f(51, 2, "team-offsite.png", false, 1.8 * MB, "image/png", 2, 28),
    f(52, 2, "sunset-timelapse.mp4", false, 68 * MB, "video/mp4", 5, 26),
    f(53, 2, "profile.png", false, 240_128, "image/png", 3, 12),

    // Projects/
    f(60, 3, "unidisk", true, 0, "", undefined, 70),
    f(61, 3, "roadmap.md", false, 5_600, "text/markdown", 4, 5,
      "# Roadmap\n\n## Shipped\n- Round-robin routing across accounts\n- Presigned share links\n- Scoped API keys\n\n## Considering\n- Client-side encryption before upload\n- Chunked files spanning multiple accounts\n- WebDAV endpoint\n"),

    // Projects/unidisk/
    f(70, 60, "docker-compose.yml", false, 1_284, "text/yaml", 1, 11,
      `services:\n  unidisk:\n    image: ghcr.io/kayamii/unidisk:latest\n    ports:\n      - "8080:8080"\n    volumes:\n      - unidisk-data:/data\n    environment:\n      UNIDISK_ADMIN_EMAIL: admin@example.com\n      UNIDISK_ADMIN_PASSWORD: change-me\n\nvolumes:\n  unidisk-data:\n`),
    f(71, 60, "notes.txt", false, 940, "text/plain", 3, 8,
      "Stream-through means the server never buffers a whole file to disk.\nBytes go: browser -> UniDisk -> provider, in one pass.\n"),

    // Backups/
    f(80, 4, "db-2026-08-14.sql.gz", false, 312 * MB, "application/gzip", 5, 1),
    f(81, 4, "db-2026-08-13.sql.gz", false, 309 * MB, "application/gzip", 5, 2),
    f(82, 4, "db-2026-08-12.sql.gz", false, 311 * MB, "application/gzip", 5, 3),
  ];
}
