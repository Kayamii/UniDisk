// Typed client for the UniDisk backend. The token is kept in localStorage and
// attached as a bearer header on every request.

// Privilege strings mirror the backend's store.Privilege constants.
export type Privilege =
  | "files.view"
  | "files.upload"
  | "files.download"
  | "files.delete"
  | "providers.manage"
  | "users.manage"
  | "roles.manage"
  | "settings.manage";

export interface User {
  id: number;
  email: string;
  role_id: number | null;
  role_name: string;
  must_change_password: boolean;
  privileges: Privilege[];
  created_at: string;
}

export interface Role {
  id: number;
  name: string;
  is_system: boolean;
  privileges: Privilege[];
}

export interface Account {
  id: number;
  provider: string;
  display_name: string;
  status: string;
  quota_bytes: number;
  used_bytes: number;
  priority: number;
  created_at: string;
}

export interface FileNode {
  id: number;
  parent_id: number | null;
  name: string;
  is_dir: boolean;
  size_bytes: number;
  mime_type: string;
  account_id?: number;
  created_at: string;
  updated_at: string;
}

export interface CredentialField {
  key: string;
  label: string;
  type: "text" | "password" | "oauth";
  required: boolean;
  help?: string;
}

export interface ProviderDescriptor {
  name: string;
  title: string;
  fields: CredentialField[];
  oauth: boolean;
}

export interface AccountSummary {
  id: number;
  provider: string;
  display_name: string;
  status: string;
  quota_bytes: number;
  used_bytes: number;
}

export interface Settings {
  fill_threshold_pct: number;
}

export interface APIKey {
  id: number;
  name: string;
  key_prefix: string;
  privileges: Privilege[];
  created_at: string;
  expires_at: string | null;
  last_used_at: string | null;
}

export interface PresignedURL {
  id: number;
  token: string;
  file_id: number;
  file_name: string;
  created_at: string;
  expires_at: string | null;
  downloads: number;
  url: string;
}

export interface PoolStats {
  total_bytes: number;
  used_bytes: number;
  available_bytes: number;
  account_count: number;
  accounts: AccountSummary[];
}

const TOKEN_KEY = "unidisk_token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}
export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}
export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

/** ApiError carries the HTTP status alongside the server's message. */
export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown
): Promise<T> {
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  if (body !== undefined) headers["Content-Type"] = "application/json";

  const res = await fetch(path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 204) return undefined as T;

  const text = await res.text();
  const data = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    throw new ApiError(res.status, data?.error ?? `request failed (${res.status})`);
  }
  return data as T;
}

interface AuthResponse {
  token: string;
  user: User;
}

export const api = {
  // Auth
  login: (email: string, password: string) =>
    request<AuthResponse>("POST", "/api/auth/login", { email, password }),
  me: () => request<User>("GET", "/api/auth/me"),
  changePassword: (currentPassword: string, newPassword: string) =>
    request<void>("POST", "/api/auth/change-password", {
      current_password: currentPassword,
      new_password: newPassword,
    }),

  // Users & roles (admin)
  users: () => request<User[]>("GET", "/api/users"),
  createUser: (email: string, password: string, roleId: number) =>
    request<User>("POST", "/api/users", { email, password, role_id: roleId }),
  setUserRole: (id: number, roleId: number) =>
    request<void>("PUT", `/api/users/${id}/role`, { role_id: roleId }),
  resetUserPassword: (id: number, password: string) =>
    request<void>("PUT", `/api/users/${id}/password`, { password }),
  deleteUser: (id: number) => request<void>("DELETE", `/api/users/${id}`),

  roles: () => request<Role[]>("GET", "/api/roles"),
  allPrivileges: () => request<Privilege[]>("GET", "/api/privileges"),
  createRole: (name: string, privileges: Privilege[]) =>
    request<Role>("POST", "/api/roles", { name, privileges }),
  updateRole: (id: number, privileges: Privilege[]) =>
    request<Role>("PUT", `/api/roles/${id}`, { privileges }),
  deleteRole: (id: number) => request<void>("DELETE", `/api/roles/${id}`),

  // Providers & accounts
  providers: () => request<ProviderDescriptor[]>("GET", "/api/providers"),
  accounts: () => request<Account[]>("GET", "/api/accounts"),
  addAccount: (provider: string, displayName: string, credentials: Record<string, string>) =>
    request<Account>("POST", "/api/accounts", {
      provider,
      display_name: displayName,
      credentials,
    }),
  deleteAccount: (id: number) => request<void>("DELETE", `/api/accounts/${id}`),
  // Returns the provider consent URL to send the browser to for the
  // one-click "Connect" flow.
  oauthStart: (provider: string) =>
    request<{ url: string }>("GET", `/api/oauth/${provider}/start`),

  // Files
  files: (parent?: number | null) =>
    request<FileNode[]>("GET", `/api/files${parent != null ? `?parent=${parent}` : ""}`),
  searchFiles: (q: string) =>
    request<FileNode[]>("GET", `/api/files/search?q=${encodeURIComponent(q)}`),
  createFolder: (name: string, parentId: number | null) =>
    request<FileNode>("POST", "/api/files/folder", { name, parent_id: parentId }),
  renameFile: (id: number, name: string) =>
    request<void>("PUT", `/api/files/${id}`, { name }),
  deleteFile: (id: number) => request<void>("DELETE", `/api/files/${id}`),

  // Stats
  stats: () => request<PoolStats>("GET", "/api/stats"),

  // Settings (routing threshold)
  getSettings: () => request<Settings>("GET", "/api/settings"),
  updateSettings: (s: Settings) => request<Settings>("PUT", "/api/settings", s),

  // API keys
  apiKeys: () => request<APIKey[]>("GET", "/api/keys"),
  grantableKeyPrivileges: () => request<Privilege[]>("GET", "/api/keys/grantable"),
  createApiKey: (name: string, privileges: Privilege[], expiresInDays: number) =>
    request<{ key: string; api_key: APIKey }>("POST", "/api/keys", {
      name, privileges, expires_in_days: expiresInDays,
    }),
  deleteApiKey: (id: number) => request<void>("DELETE", `/api/keys/${id}`),

  // Presigned share links
  presigned: () => request<PresignedURL[]>("GET", "/api/presigned"),
  createPresigned: (fileId: number, expiresInHours: number) =>
    request<PresignedURL>("POST", "/api/presigned", {
      file_id: fileId, expires_in_hours: expiresInHours,
    }),
  deletePresigned: (id: number) => request<void>("DELETE", `/api/presigned/${id}`),

  /**
   * Upload streams a File to the backend (which streams it to a provider).
   * Uses XMLHttpRequest because fetch() cannot report upload progress.
   * onProgress receives loaded/total bytes and bytes-per-second.
   */
  upload(
    file: File,
    parent: number | null,
    onProgress?: (p: { loaded: number; total: number; bps: number }) => void
  ): Promise<FileNode> {
    return new Promise<FileNode>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      const qs = parent != null ? `?parent=${parent}` : "";
      xhr.open("POST", `/api/files/upload${qs}`);
      xhr.setRequestHeader("X-Filename", encodeURIComponent(file.name));
      xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");
      const token = getToken();
      if (token) xhr.setRequestHeader("Authorization", `Bearer ${token}`);

      const startedAt = Date.now();
      xhr.upload.onprogress = (e) => {
        if (!onProgress || !e.lengthComputable) return;
        const seconds = Math.max(0.001, (Date.now() - startedAt) / 1000);
        onProgress({ loaded: e.loaded, total: e.total, bps: e.loaded / seconds });
      };
      xhr.onload = () => {
        const data = xhr.responseText ? JSON.parse(xhr.responseText) : undefined;
        if (xhr.status >= 200 && xhr.status < 300) resolve(data as FileNode);
        else reject(new ApiError(xhr.status, data?.error ?? "upload failed"));
      };
      xhr.onerror = () => reject(new ApiError(0, "network error during upload"));
      xhr.send(file);
    });
  },

  /**
   * previewBlob fetches a file inline (authenticated) and returns an object URL
   * plus its content type, for in-app preview. Caller must revokeObjectURL the
   * url when done to free memory.
   */
  async previewBlob(id: number): Promise<{ url: string; type: string }> {
    const headers: Record<string, string> = {};
    const token = getToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;
    const res = await fetch(`/api/files/${id}/download?inline=1`, { headers });
    if (!res.ok) {
      const text = await res.text();
      throw new ApiError(res.status, text || "preview failed");
    }
    const blob = await res.blob();
    return { url: URL.createObjectURL(blob), type: res.headers.get("Content-Type") || blob.type };
  },

  /**
   * download fetches a file with the bearer token and triggers a browser save.
   * It reads the response as a stream so we can report progress while bytes
   * arrive (a plain <a href> can't send the Authorization header anyway).
   * onProgress receives loaded/total bytes (total may be 0 if unknown).
   */
  async download(
    id: number,
    name: string,
    onProgress?: (p: { loaded: number; total: number }) => void
  ): Promise<void> {
    const headers: Record<string, string> = {};
    const token = getToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;
    const res = await fetch(`/api/files/${id}/download`, { headers });
    if (!res.ok) {
      const text = await res.text();
      throw new ApiError(res.status, text || "download failed");
    }

    const total = Number(res.headers.get("Content-Length") || 0);
    let blob: Blob;
    if (res.body && onProgress) {
      const reader = res.body.getReader();
      const chunks: BlobPart[] = [];
      let loaded = 0;
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        chunks.push(value);
        loaded += value.length;
        onProgress({ loaded, total });
      }
      blob = new Blob(chunks);
    } else {
      blob = await res.blob();
    }

    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },
};
