/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** "1" in the public demo build, which runs without a backend. */
  readonly VITE_DEMO?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
