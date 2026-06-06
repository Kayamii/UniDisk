# Changelog

All notable changes to UniDisk are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
[Semantic Versioning](https://semver.org/).

## [1.0.0] — 2026-06-06

First stable release. 🎉

### Storage providers
- Unified storage pool across **6 provider types**: Google Drive, Dropbox,
  OneDrive, Box, pCloud, and any **S3-compatible** store (Amazon S3, Backblaze
  B2, Wasabi, Cloudflare R2, MinIO, DigitalOcean Spaces).
- Connect **unlimited accounts** per provider — capacity grows as you add more.
- One-click OAuth **"Connect"** flow for the cloud providers, plus a manual
  credential form for S3. Credentials are verified live before saving.
- **Round-robin routing** with a configurable fill threshold; falls back to the
  account with the most free space when all are near full.

### File management
- Folders, **drag-and-drop upload**, search, grid/list views, rename, delete.
- **Stream-through** transfers (no server-side disk buffering) with live upload
  speed and download progress.
- In-app **preview** for images, PDF, video, audio, and text/code.
- File-type icons.

### Access control & sharing
- **RBAC** with built-in Admin and Viewer roles plus custom roles and granular
  privileges. Admin-managed users with forced first-login password change. No
  open sign-up; the first admin is seeded from the environment.
- **Scoped API keys** (file permissions only, never exceeding the owner) for
  programmatic upload/download.
- **Presigned share links** — public, expiring, direct-download URLs.

### Security
- Provider credentials are **AES-256-GCM encrypted at rest**.
- Auto-detects its public URL, so links and OAuth redirects work by IP, by DNS,
  or behind a reverse proxy with no configuration.
- CI: build/test, `govulncheck`, CodeQL, and Trivy container scanning.

### Deployment
- Single container (Go API + React SPA + SQLite). Multi-arch images
  (`linux/amd64`, `linux/arm64`) on GHCR and Docker Hub.

[1.0.0]: https://github.com/Kayamii/UniDisk/releases/tag/v1.0.0
