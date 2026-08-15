// An in-browser stand-in for the UniDisk Go backend, used only by the public
// demo build (VITE_DEMO=1). It answers the same paths with the same shapes, so
// no page or component needs to know it exists.
//
// Behaviour differences from the real backend are deliberate and limited to:
//   - credentials are never verified (any provider "connects" successfully)
//   - OAuth redirects are simulated locally instead of leaving the page
//   - file bytes live in the tab, not in a cloud account

import { ApiError, type Account, type APIKey, type FileNode, type PresignedURL,
  type Privilege, type ProviderDescriptor, type Role, type User } from "../api";
import { DEMO_PASSWORD } from "./credentials";
import { ALL_PRIVILEGES, DEMO_ADMIN, GB } from "./seed";
import { db, getBlob, nextId, persist, putBlob } from "./store";

/** Simulated latency, so loading states and skeletons are actually visible. */
function delay<T>(value: T, ms = 180): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(value), ms));
}

function now(): string {
  return new Date().toISOString();
}

function currentUser(): User {
  const s = db().session;
  const user = db().users.find((u) => u.email === s);
  if (!user) throw new ApiError(401, "not authenticated");
  return user;
}

// Provider descriptors mirror backend/internal/provider/*. OAuth providers show
// a Connect button; s3 renders its credential form.
const PROVIDERS: ProviderDescriptor[] = [
  { name: "googledrive", title: "Google Drive", oauth: true, fields: [] },
  { name: "dropbox", title: "Dropbox", oauth: true, fields: [] },
  { name: "onedrive", title: "OneDrive", oauth: true, fields: [] },
  { name: "box", title: "Box", oauth: true, fields: [] },
  { name: "pcloud", title: "pCloud", oauth: true, fields: [] },
  {
    name: "s3", title: "S3-compatible", oauth: false,
    fields: [
      { key: "access_key_id", label: "Access Key ID", type: "text", required: true },
      { key: "secret_access_key", label: "Secret Access Key", type: "password", required: true },
      { key: "bucket", label: "Bucket", type: "text", required: true },
      { key: "region", label: "Region", type: "text", required: true,
        help: "e.g. us-east-1. For Backblaze/Wasabi/R2 use their region." },
      { key: "endpoint", label: "Endpoint URL", type: "text", required: false,
        help: "Leave blank for AWS S3. For others, e.g. https://s3.us-west-001.backblazeb2.com" },
    ],
  },
];

/** Default quota for a newly "connected" account, by provider. */
const DEFAULT_QUOTA: Record<string, number> = {
  googledrive: 15 * GB,
  dropbox: 2 * GB,
  onedrive: 5 * GB,
  box: 10 * GB,
  pcloud: 10 * GB,
  s3: 50 * GB,
};

/**
 * pickAccount mirrors the real router: round-robin across accounts below the
 * fill threshold, else the one with the most free space.
 */
function pickAccount(): Account | undefined {
  const { accounts, settings } = db();
  const active = accounts.filter((a) => a.status === "active");
  if (active.length === 0) return undefined;
  const eligible = active.filter(
    (a) => a.quota_bytes === 0 || (a.used_bytes / a.quota_bytes) * 100 < settings.fill_threshold_pct
  );
  const pool = eligible.length > 0 ? eligible : active;
  if (eligible.length > 0) {
    // Round-robin: least recently written wins, approximated by lowest usage
    // ratio so repeated uploads visibly spread across accounts.
    return pool.reduce((best, a) => {
      const ratio = (x: Account) => (x.quota_bytes > 0 ? x.used_bytes / x.quota_bytes : 0);
      return ratio(a) < ratio(best) ? a : best;
    });
  }
  return pool.reduce((best, a) =>
    a.quota_bytes - a.used_bytes > best.quota_bytes - best.used_bytes ? a : best
  );
}

/** collectDescendants returns a node's id plus every id beneath it. */
function collectDescendants(id: number): number[] {
  const out = [id];
  const queue = [id];
  while (queue.length) {
    const parent = queue.shift()!;
    for (const f of db().files) {
      if (f.parent_id === parent) {
        out.push(f.id);
        queue.push(f.id);
      }
    }
  }
  return out;
}

function stripBody(f: FileNode & { body?: string }): FileNode {
  const { body: _body, ...rest } = f as FileNode & { body?: string };
  return rest;
}

/** Files sort folders first, then by name — same as the real listing. */
function sortNodes(a: FileNode, b: FileNode): number {
  if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
  return a.name.localeCompare(b.name);
}

function keyPrefix(): string {
  return "udk_" + Math.random().toString(36).slice(2, 6);
}

function token(len: number): string {
  const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let out = "";
  for (let i = 0; i < len; i++) out += chars[Math.floor(Math.random() * chars.length)];
  return out;
}

/**
 * handle routes a method+path to the demo state. It throws ApiError exactly
 * where the real backend would, so error paths in the UI stay exercised.
 */
export async function handle<T>(method: string, rawPath: string, body?: unknown): Promise<T> {
  const [path, search] = rawPath.split("?");
  const params = new URLSearchParams(search ?? "");
  const state = db();
  const b = (body ?? {}) as Record<string, never>;
  const send = (v: unknown) => delay(v) as Promise<T>;

  // --- Auth ---------------------------------------------------------------
  if (method === "POST" && path === "/api/auth/login") {
    const email = String((b as Record<string, unknown>).email ?? "");
    const password = String((b as Record<string, unknown>).password ?? "");
    const user = state.users.find((u) => u.email.toLowerCase() === email.toLowerCase());
    if (!user || password !== DEMO_PASSWORD) {
      throw new ApiError(401, `invalid credentials — this demo accepts any listed user with the password "${DEMO_PASSWORD}"`);
    }
    state.session = user.email;
    persist();
    return send({ token: "demo-token", user });
  }

  if (method === "GET" && path === "/api/auth/me") return send(currentUser());

  if (method === "POST" && path === "/api/auth/change-password") {
    const user = currentUser();
    user.must_change_password = false;
    persist();
    return send(undefined);
  }

  // --- Users --------------------------------------------------------------
  if (method === "GET" && path === "/api/users") return send(state.users);

  if (method === "POST" && path === "/api/users") {
    const email = String((b as Record<string, unknown>).email ?? "");
    const roleId = Number((b as Record<string, unknown>).role_id);
    if (state.users.some((u) => u.email.toLowerCase() === email.toLowerCase())) {
      throw new ApiError(409, "a user with that email already exists");
    }
    const role = state.roles.find((r) => r.id === roleId);
    const user: User = {
      id: nextId(), email, role_id: roleId,
      role_name: role?.name ?? "Viewer",
      must_change_password: true,
      privileges: role?.privileges ?? [],
      created_at: now(),
    };
    state.users.push(user);
    persist();
    return send(user);
  }

  const userRole = path.match(/^\/api\/users\/(\d+)\/role$/);
  if (method === "PUT" && userRole) {
    const user = state.users.find((u) => u.id === Number(userRole[1]));
    if (!user) throw new ApiError(404, "user not found");
    const role = state.roles.find((r) => r.id === Number((b as Record<string, unknown>).role_id));
    if (!role) throw new ApiError(404, "role not found");
    user.role_id = role.id;
    user.role_name = role.name;
    user.privileges = [...role.privileges];
    persist();
    return send(undefined);
  }

  const userPassword = path.match(/^\/api\/users\/(\d+)\/password$/);
  if (method === "PUT" && userPassword) {
    const user = state.users.find((u) => u.id === Number(userPassword[1]));
    if (!user) throw new ApiError(404, "user not found");
    user.must_change_password = true;
    persist();
    return send(undefined);
  }

  const userId = path.match(/^\/api\/users\/(\d+)$/);
  if (method === "DELETE" && userId) {
    const id = Number(userId[1]);
    if (id === DEMO_ADMIN.id) throw new ApiError(400, "cannot delete the last admin");
    state.users = state.users.filter((u) => u.id !== id);
    persist();
    return send(undefined);
  }

  // --- Roles --------------------------------------------------------------
  if (method === "GET" && path === "/api/roles") return send(state.roles);
  if (method === "GET" && path === "/api/privileges") return send(ALL_PRIVILEGES);

  if (method === "POST" && path === "/api/roles") {
    const role: Role = {
      id: nextId(),
      name: String((b as Record<string, unknown>).name ?? "New role"),
      is_system: false,
      privileges: ((b as Record<string, unknown>).privileges as Privilege[]) ?? [],
    };
    state.roles.push(role);
    persist();
    return send(role);
  }

  const roleId = path.match(/^\/api\/roles\/(\d+)$/);
  if (method === "PUT" && roleId) {
    const role = state.roles.find((r) => r.id === Number(roleId[1]));
    if (!role) throw new ApiError(404, "role not found");
    if (role.is_system) throw new ApiError(400, "system roles cannot be modified");
    role.privileges = ((b as Record<string, unknown>).privileges as Privilege[]) ?? [];
    // Keep users of this role in sync so the nav updates immediately.
    for (const u of state.users) {
      if (u.role_id === role.id) u.privileges = [...role.privileges];
    }
    persist();
    return send(role);
  }
  if (method === "DELETE" && roleId) {
    const id = Number(roleId[1]);
    const role = state.roles.find((r) => r.id === id);
    if (role?.is_system) throw new ApiError(400, "system roles cannot be deleted");
    if (state.users.some((u) => u.role_id === id)) {
      throw new ApiError(400, "role is still assigned to a user");
    }
    state.roles = state.roles.filter((r) => r.id !== id);
    persist();
    return send(undefined);
  }

  // --- Providers & accounts ----------------------------------------------
  if (method === "GET" && path === "/api/providers") return send(PROVIDERS);
  if (method === "GET" && path === "/api/accounts") return send(state.accounts);

  if (method === "POST" && path === "/api/accounts") {
    const provider = String((b as Record<string, unknown>).provider ?? "s3");
    const creds = ((b as Record<string, unknown>).credentials ?? {}) as Record<string, string>;
    // The real backend verifies credentials here; the demo only checks that
    // required fields are filled, so the failure path is still reachable.
    const descriptor = PROVIDERS.find((p) => p.name === provider);
    for (const field of descriptor?.fields ?? []) {
      if (field.required && !creds[field.key]?.trim()) {
        throw new ApiError(400, `${field.label} is required`);
      }
    }
    const account = addAccount(
      provider,
      String((b as Record<string, unknown>).display_name ?? "") ||
        `${descriptor?.title ?? provider} · ${creds.bucket ?? "demo"}`
    );
    return send(account);
  }

  const accountId = path.match(/^\/api\/accounts\/(\d+)$/);
  if (method === "DELETE" && accountId) {
    state.accounts = state.accounts.filter((a) => a.id !== Number(accountId[1]));
    persist();
    return send(undefined);
  }

  const oauthStart = path.match(/^\/api\/oauth\/([a-z0-9]+)\/start$/);
  if (method === "GET" && oauthStart) {
    // No real consent screen in the demo: connect immediately and send the user
    // back to /providers with the same ?connected= marker the backend uses.
    const provider = oauthStart[1];
    const descriptor = PROVIDERS.find((p) => p.name === provider);
    const n = state.accounts.filter((a) => a.provider === provider).length + 1;
    addAccount(provider, `${descriptor?.title ?? provider} · demo ${n}`);
    return send({ url: `${location.origin}${location.pathname}#/providers?connected=${descriptor?.title ?? provider}` });
  }

  // --- Files --------------------------------------------------------------
  if (method === "GET" && path === "/api/files/search") {
    const q = (params.get("q") ?? "").toLowerCase();
    const hits = state.files
      .filter((f) => f.name.toLowerCase().includes(q))
      .map(stripBody)
      .sort(sortNodes);
    return send(hits);
  }

  if (method === "GET" && path === "/api/files") {
    const parent = params.has("parent") ? Number(params.get("parent")) : null;
    const listing = state.files
      .filter((f) => (f.parent_id ?? null) === parent)
      .map(stripBody)
      .sort(sortNodes);
    return send(listing);
  }

  if (method === "POST" && path === "/api/files/folder") {
    const name = String((b as Record<string, unknown>).name ?? "").trim();
    if (!name) throw new ApiError(400, "name is required");
    const parentId = ((b as Record<string, unknown>).parent_id ?? null) as number | null;
    if (state.files.some((f) => (f.parent_id ?? null) === parentId && f.name === name)) {
      throw new ApiError(409, "a file with that name already exists here");
    }
    const folder: FileNode = {
      id: nextId(), parent_id: parentId, name, is_dir: true,
      size_bytes: 0, mime_type: "", created_at: now(), updated_at: now(),
    };
    state.files.push(folder);
    persist();
    return send(folder);
  }

  const fileId = path.match(/^\/api\/files\/(\d+)$/);
  if (method === "PUT" && fileId) {
    const file = state.files.find((f) => f.id === Number(fileId[1]));
    if (!file) throw new ApiError(404, "file not found");
    const name = String((b as Record<string, unknown>).name ?? "").trim();
    if (!name) throw new ApiError(400, "name is required");
    file.name = name;
    file.updated_at = now();
    persist();
    return send(undefined);
  }
  if (method === "DELETE" && fileId) {
    const id = Number(fileId[1]);
    const doomed = new Set(collectDescendants(id));
    // Give the freed space back to the accounts that held the bytes.
    for (const f of state.files) {
      if (doomed.has(f.id) && !f.is_dir && f.account_id) {
        const account = state.accounts.find((a) => a.id === f.account_id);
        if (account) account.used_bytes = Math.max(0, account.used_bytes - f.size_bytes);
      }
    }
    state.files = state.files.filter((f) => !doomed.has(f.id));
    state.presigned = state.presigned.filter((p) => !doomed.has(p.file_id));
    persist();
    return send(undefined);
  }

  // --- Stats & settings ---------------------------------------------------
  if (method === "GET" && path === "/api/stats") {
    const accounts = state.accounts;
    const total = accounts.reduce((s, a) => s + a.quota_bytes, 0);
    const used = accounts.reduce((s, a) => s + a.used_bytes, 0);
    return send({
      total_bytes: total,
      used_bytes: used,
      available_bytes: Math.max(0, total - used),
      account_count: accounts.length,
      accounts: accounts.map((a) => ({
        id: a.id, provider: a.provider, display_name: a.display_name,
        status: a.status, quota_bytes: a.quota_bytes, used_bytes: a.used_bytes,
      })),
    });
  }

  if (method === "GET" && path === "/api/settings") return send(state.settings);
  if (method === "PUT" && path === "/api/settings") {
    const pct = Number((b as Record<string, unknown>).fill_threshold_pct);
    if (!Number.isFinite(pct) || pct < 1 || pct > 100) {
      throw new ApiError(400, "fill threshold must be between 1 and 100");
    }
    state.settings = { fill_threshold_pct: pct };
    persist();
    return send(state.settings);
  }

  // --- API keys -----------------------------------------------------------
  if (method === "GET" && path === "/api/keys") return send(state.apiKeys);
  if (method === "GET" && path === "/api/keys/grantable") {
    return send(currentUser().privileges.filter((p) => p.startsWith("files.")));
  }
  if (method === "POST" && path === "/api/keys") {
    const days = Number((b as Record<string, unknown>).expires_in_days) || 0;
    const apiKey: APIKey = {
      id: nextId(),
      name: String((b as Record<string, unknown>).name ?? "untitled"),
      key_prefix: keyPrefix(),
      privileges: ((b as Record<string, unknown>).privileges as Privilege[]) ?? [],
      created_at: now(),
      expires_at: days > 0 ? new Date(Date.now() + days * 86_400_000).toISOString() : null,
      last_used_at: null,
    };
    state.apiKeys.push(apiKey);
    persist();
    return send({ key: `${apiKey.key_prefix}${token(28)}`, api_key: apiKey });
  }
  const keyId = path.match(/^\/api\/keys\/(\d+)$/);
  if (method === "DELETE" && keyId) {
    state.apiKeys = state.apiKeys.filter((k) => k.id !== Number(keyId[1]));
    persist();
    return send(undefined);
  }

  // --- Presigned links ----------------------------------------------------
  if (method === "GET" && path === "/api/presigned") return send(state.presigned);
  if (method === "POST" && path === "/api/presigned") {
    const id = Number((b as Record<string, unknown>).file_id);
    const file = state.files.find((f) => f.id === id);
    if (!file) throw new ApiError(404, "file not found");
    const hours = Number((b as Record<string, unknown>).expires_in_hours) || 0;
    const t = token(12);
    const link: PresignedURL = {
      id: nextId(), token: t, file_id: file.id, file_name: file.name,
      created_at: now(),
      expires_at: hours > 0 ? new Date(Date.now() + hours * 3_600_000).toISOString() : null,
      downloads: 0,
      url: `${location.origin}${import.meta.env.BASE_URL}s/${t}`,
    };
    state.presigned.push(link);
    persist();
    return send(link);
  }
  const presignedId = path.match(/^\/api\/presigned\/(\d+)$/);
  if (method === "DELETE" && presignedId) {
    state.presigned = state.presigned.filter((p) => p.id !== Number(presignedId[1]));
    persist();
    return send(undefined);
  }

  throw new ApiError(404, `demo backend has no route for ${method} ${path}`);
}

/** addAccount creates and stores a connected account for a provider. */
function addAccount(provider: string, displayName: string): Account {
  const account: Account = {
    id: nextId(),
    provider,
    display_name: displayName,
    status: "active",
    quota_bytes: DEFAULT_QUOTA[provider] ?? 10 * GB,
    used_bytes: 0,
    priority: 0,
    created_at: now(),
  };
  db().accounts.push(account);
  persist();
  return account;
}

/**
 * demoUpload records a file and animates progress at a believable speed, so the
 * upload UI (progress bar, bytes/sec) behaves as it does against a real server.
 */
export function demoUpload(
  file: File,
  parent: number | null,
  onProgress?: (p: { loaded: number; total: number; bps: number }) => void
): Promise<FileNode> {
  return new Promise((resolve, reject) => {
    const state = db();
    if (state.files.some((f) => (f.parent_id ?? null) === parent && f.name === file.name)) {
      reject(new ApiError(409, "a file with that name already exists here"));
      return;
    }
    const account = pickAccount();
    if (!account) {
      reject(new ApiError(400, "no storage accounts connected — add one from Providers"));
      return;
    }

    const total = file.size;
    const bps = 12 * 1024 * 1024; // pretend ~12 MB/s
    const durationMs = Math.min(4000, Math.max(600, (total / bps) * 1000));
    const startedAt = Date.now();

    const tick = () => {
      const elapsed = Date.now() - startedAt;
      const ratio = Math.min(1, elapsed / durationMs);
      const loaded = Math.round(total * ratio);
      onProgress?.({
        loaded,
        total,
        bps: elapsed > 0 ? loaded / (elapsed / 1000) : 0,
      });
      if (ratio < 1) {
        requestAnimationFrame(tick);
        return;
      }

      const node: FileNode = {
        id: nextId(),
        parent_id: parent,
        name: file.name,
        is_dir: false,
        size_bytes: total,
        mime_type: file.type || "application/octet-stream",
        account_id: account.id,
        created_at: now(),
        updated_at: now(),
      };
      state.files.push(node);
      account.used_bytes += total;
      putBlob(node.id, file);
      persist();
      resolve(node);
    };
    requestAnimationFrame(tick);
  });
}

/**
 * demoBlob resolves a file's contents for preview/download. Files the visitor
 * uploaded in this session return their real bytes; seeded files return their
 * inlined text, a generated placeholder image, or a short note explaining that
 * the bytes live in a real provider in a real deployment.
 */
export async function demoBlob(id: number): Promise<{ blob: Blob; type: string }> {
  const uploaded = getBlob(id);
  if (uploaded) return { blob: uploaded, type: uploaded.type || "application/octet-stream" };

  const file = db().files.find((f) => f.id === id);
  if (!file) throw new ApiError(404, "file not found");

  if (file.body != null) {
    return { blob: new Blob([file.body], { type: file.mime_type }), type: file.mime_type };
  }
  if (file.mime_type.startsWith("image/")) {
    const svg = placeholderImage(file.name);
    return { blob: new Blob([svg], { type: "image/svg+xml" }), type: "image/svg+xml" };
  }
  const note =
    `${file.name}\n\n` +
    `This is the UniDisk demo, which runs entirely in your browser.\n` +
    `Seeded files are metadata only — in a real deployment the bytes for this\n` +
    `file would stream from the provider account it was routed to.\n\n` +
    `Upload a file of your own and it will preview and download for real.\n`;
  return { blob: new Blob([note], { type: "text/plain" }), type: "text/plain" };
}

/** placeholderImage draws a labelled tile so image previews aren't empty. */
function placeholderImage(name: string): string {
  const safe = name.replace(/[<>&]/g, "");
  return `<svg xmlns="http://www.w3.org/2000/svg" width="800" height="500" viewBox="0 0 800 500">
  <defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0%" stop-color="#1e293b"/><stop offset="100%" stop-color="#0f172a"/>
  </linearGradient></defs>
  <rect width="800" height="500" fill="url(#g)"/>
  <text x="400" y="250" fill="#94a3b8" font-family="system-ui, sans-serif" font-size="26"
    text-anchor="middle">${safe}</text>
  <text x="400" y="288" fill="#475569" font-family="system-ui, sans-serif" font-size="16"
    text-anchor="middle">demo placeholder — upload your own file to preview it</text>
</svg>`;
}
