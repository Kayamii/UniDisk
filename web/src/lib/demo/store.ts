// Persistent state for the demo build.
//
// The demo has no backend. Everything a visitor does — uploading, renaming,
// creating users, connecting a fake provider — is written to this store, which
// mirrors itself into localStorage so a page reload keeps the changes. Nothing
// leaves the browser.
//
// Uploaded file *bytes* are held in memory only (a Map of Blobs), because
// localStorage can't hold them; metadata survives a reload, and a re-opened
// preview of a file uploaded in a previous session falls back gracefully.

import type {
  Account, APIKey, PresignedURL, Role, Settings, User,
} from "../api";
import {
  seedAccounts, seedApiKeys, seedFiles, seedPresigned, seedRoles,
  seedSettings, seedUsers, type SeedFile,
} from "./seed";

const STORAGE_KEY = "unidisk_demo_state_v1";

export interface DemoState {
  users: User[];
  roles: Role[];
  accounts: Account[];
  files: SeedFile[];
  apiKeys: APIKey[];
  presigned: PresignedURL[];
  settings: Settings;
  /** Highest id handed out so far; new rows take nextId++. */
  nextId: number;
  /** Email of the signed-in demo user, or null when signed out. */
  session: string | null;
}

function freshState(): DemoState {
  return {
    users: seedUsers(),
    roles: seedRoles(),
    accounts: seedAccounts(),
    files: seedFiles(),
    apiKeys: seedApiKeys(),
    presigned: seedPresigned(),
    settings: seedSettings(),
    nextId: 1000,
    session: null,
  };
}

let state: DemoState = load();

function load(): DemoState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return freshState();
    const parsed = JSON.parse(raw) as DemoState;
    // Tolerate older/partial shapes rather than showing a broken demo.
    return { ...freshState(), ...parsed };
  } catch {
    return freshState();
  }
}

/** persist writes the current state back to localStorage (best-effort). */
export function persist() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // Quota exceeded or storage disabled — the demo still works in-memory.
  }
}

export function db(): DemoState {
  return state;
}

export function nextId(): number {
  state.nextId += 1;
  persist();
  return state.nextId;
}

/** reset restores the seeded state — wired to the "Reset demo" button. */
export function resetDemo() {
  state = freshState();
  blobs.clear();
  persist();
}

// --- Uploaded file contents (in-memory only) --------------------------------

const blobs = new Map<number, Blob>();

export function putBlob(id: number, blob: Blob) {
  blobs.set(id, blob);
}

export function getBlob(id: number): Blob | undefined {
  return blobs.get(id);
}
