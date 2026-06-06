# Contributing to UniDisk

Thanks for your interest in improving UniDisk! Contributions of all kinds are
welcome — bug reports, features, docs, and new storage providers.

## Development setup

Requirements: **Go 1.24+**, **Node 20+**, and (optionally) **Docker**.

```bash
# Backend — serves the API on :8080
cd backend
UNIDISK_DATA_DIR=./data UNIDISK_ADMIN_EMAIL=admin@local UNIDISK_ADMIN_PASSWORD=changeme123 \
  go run ./cmd/unidisk

# Frontend — Vite dev server on :5173, proxies /api to :8080
cd web
npm install
npm run dev
```

Open <http://localhost:5173>.

## Before opening a pull request

```bash
# Backend
cd backend && go build ./... && go vet ./... && go test ./...

# Frontend
cd web && npm run build   # runs the TypeScript type-check + production build
```

Please keep PRs focused, match the existing code style, and update docs when
behavior changes.

## Adding a storage provider

UniDisk providers implement a single interface — see
[`backend/internal/provider/provider.go`](backend/internal/provider/provider.go).

1. Create `backend/internal/provider/<name>/<name>.go`.
2. Implement `provider.Provider` (and `provider.OAuthProvider` if it uses a
   browser "Connect" flow). Existing providers are good templates:
   - OAuth flow: `dropbox`, `googledrive`, `onedrive`, `box`, `pcloud`
   - Manual credentials: `s3`
3. Register it in [`backend/cmd/unidisk/main.go`](backend/cmd/unidisk/main.go).
4. Add any instance config (OAuth app id/secret) to `internal/config/config.go`
   and `docker-compose.yml`.

No frontend changes are needed — the dashboard renders each provider's form
from its declared credential schema.

## Reporting bugs / requesting features

Use the issue templates. Include steps to reproduce, expected vs. actual
behavior, and your deployment method (Docker, bare metal, reverse proxy, …).
