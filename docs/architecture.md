# Architecture

How UniDisk turns several ordinary cloud accounts into one drive, and what it
does — and deliberately does not — keep.

## The one-line version

UniDisk is a **metadata layer with a router in front of it**. It remembers which
provider account holds each file; when bytes move, they stream straight through
the server without ever landing on its disk.

## Components

```mermaid
flowchart TB
    subgraph client["Browser"]
        SPA["React SPA<br/>TypeScript · Tailwind · shadcn/ui"]
    end

    subgraph server["UniDisk container (single Go binary)"]
        API["HTTP API<br/>internal/api"]
        RBAC["Auth & RBAC<br/>JWT · API keys · privileges"]
        POOL["Pool router<br/>internal/pool"]
        CRYPTO["Credential vault<br/>AES-256-GCM"]
        DB[("SQLite<br/>users · roles · file tree<br/>encrypted credentials")]
        REG["Provider registry<br/>internal/provider"]
    end

    subgraph providers["Your cloud accounts"]
        GD["Google Drive"]
        DB2["Dropbox"]
        OD["OneDrive"]
        BOX["Box"]
        PC["pCloud"]
        S3["S3-compatible"]
    end

    SPA -->|"/api/*"| API
    API --> RBAC
    RBAC --> POOL
    POOL --> DB
    POOL --> REG
    CRYPTO --- DB
    REG --> GD & DB2 & OD & BOX & PC & S3

    classDef store fill:#0f172a,stroke:#334155,color:#e2e8f0
    class DB store
```

Every provider is one package implementing a shared interface, so adding a
seventh is a self-contained change — the router, the API, and the credential
form all adapt from the interface and the provider's declared schema.

## Where a file actually goes

An upload never touches UniDisk's disk. The server opens a reader on the request
body and a writer to the chosen provider, then copies between them.

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant S as UniDisk
    participant R as Pool router
    participant D as SQLite
    participant P as Provider account

    U->>S: POST /api/files/upload (stream)
    S->>S: Authenticate (JWT or API key), check files.upload
    S->>R: Which account has room?
    R->>D: Read account usage + fill threshold
    R-->>S: Account #3 (round-robin, under threshold)
    S->>D: Read + decrypt account credentials
    S->>P: Stream bytes through (no disk buffering)
    P-->>S: Provider object id
    S->>D: Record name, size, parent, account, object id
    S-->>U: 201 with the new file node
```

Downloads run the same path in reverse: look up which account holds the file,
open a stream from that provider, pipe it to the client.

## Routing

Uploads are spread **round-robin across every account below a fill threshold**
(80% by default, configurable under Storage Pool → Routing). When every account
is above the threshold, the file goes to whichever has the most free space.

```
accounts under threshold?  ──yes──►  round-robin among them
         │
         no
         ▼
   most free space wins
```

The practical effect: connecting another free account immediately grows the
pool, and new uploads start landing there without any migration.

## What is stored where

| Data | Location | Notes |
| --- | --- | --- |
| File **contents** | Your provider accounts | Never written to UniDisk's disk |
| File **metadata** (name, size, parent, which account) | SQLite in `/data` | The map from your file tree to provider objects |
| Users, roles, privileges | SQLite | Admin-managed; no open sign-up |
| Provider credentials & OAuth tokens | SQLite, **AES-256-GCM encrypted** | Key auto-generated and persisted in `/data` |
| API keys | SQLite, hashed | Shown once at creation |
| Share links | SQLite | Token, target file, expiry, download count |

Because `/data` holds both the database and the encryption key, it is the one
thing to back up — and the one thing to protect.

## Access control

Two layers guard every request:

1. **Identity** — either a session JWT (from `/api/auth/login`) or an API key
   (`X-API-Key: udk_…`).
2. **Privilege** — each endpoint requires one of `files.view`, `files.upload`,
   `files.download`, `files.delete`, `providers.manage`, `users.manage`,
   `roles.manage`, `settings.manage`.

Roles bundle privileges; `Admin` and `Viewer` ship built in and custom roles are
composed by ticking boxes. An API key can never hold a privilege its creator
lacks, so a key is always a narrowing of its owner's access — never an
escalation.

Presigned share links are the one unauthenticated path: `GET /s/{token}` serves
a single file until its expiry, and can be revoked at any time.

## Deployment shape

One multi-stage `Dockerfile` builds the SPA with Vite, compiles the Go binary,
and ships a single container that serves both the API and the static assets from
the same origin — so there is no CORS to configure and no second service to run.

The server detects its own public URL from the request (honoring
`X-Forwarded-Proto` / `X-Forwarded-Host`), so share links and OAuth redirects
come out right whether it is reached by IP, by domain, or through a reverse
proxy. `UNIDISK_PUBLIC_URL` overrides this when a fixed value is needed.

## The demo build

The [live demo](https://kayamii.github.io/UniDisk/) is the same React app
compiled with `VITE_DEMO=1`. In that mode the API client's single `request()`
chokepoint is redirected to an in-browser stand-in that answers the same routes
from seeded state in `localStorage` — no Go backend, no network, nothing stored
anywhere but the visitor's own browser. Production builds resolve the flag to a
constant `false`, so none of that code ships in the real image.
