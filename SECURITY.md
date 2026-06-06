# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Instead, report them privately via GitHub's
[private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
(the **Security** tab → **Report a vulnerability**), or by email to the address
listed on the maintainer's GitHub profile.

You'll receive an acknowledgement as soon as possible, and we'll work with you
on a fix and coordinated disclosure.

## Deployment hardening

UniDisk serves plain HTTP and is designed to run behind a TLS-terminating
reverse proxy. When exposing an instance publicly:

- Put it behind HTTPS (Caddy, Nginx, Traefik, or Cloudflare).
- Set a strong `UNIDISK_JWT_SECRET` and `UNIDISK_ADMIN_PASSWORD`.
- Keep provider OAuth secrets in a `.env` file (never commit them).
- Restrict who can reach the admin/management routes via your network setup.

## What UniDisk stores

UniDisk stores only **metadata and orchestration state** (users, roles, file
listings, and encrypted-at-rest provider credentials). Your file contents
remain in your connected storage providers.
