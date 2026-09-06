# Och'Idoma Heritage & Palace Platform

Official digital platform for Idoma cultural heritage, history, and Palace
communications — public cultural content plus a secured Palace Admin Portal
with a versioned publishing workflow for official announcements and decrees.

## Status

This README describes what's actually built and working today, not the full
end-state spec. Current state:

**Built and verified:**
- Go/PostgreSQL backend (auth, RBAC, public read API, admin CRUD, decree/
  announcement publish workflow with audit logging)
- Full decree lifecycle tested end to end: create → submit → approve →
  publish → archive, with corrections creating new versions rather than
  editing published ones

**Not yet built:**
- Admin portal UI (all admin actions currently go through the API directly —
  see "Admin API quick reference" below)
- Public site wired to this API (the existing static site is separate)
- Automated tests
- Demo/seed content

## Tech stack

- **Backend:** Go, [chi](https://github.com/go-chi/chi) router
- **Database:** PostgreSQL (tested against [Neon](https://neon.tech)'s
  hosted free tier; any Postgres 14+ works)
- **Auth:** JWT access tokens (15 min) + httpOnly refresh token cookies (7
  days, rotated on use), Argon2id password hashing
- **Migrations:** [golang-migrate](https://github.com/golang-migrate/migrate),
  plain SQL files in `backend/migrations/`

## Project structure

```
ochidoma-palace/
├── backend/
│   ├── cmd/
│   │   ├── server/          # main API entrypoint
│   │   └── createadmin/     # one-time CLI to bootstrap the first admin
│   ├── internal/
│   │   ├── auth/            # password hashing, JWT, refresh tokens
│   │   ├── audit/           # audit log writer
│   │   ├── config/          # env var loading
│   │   ├── db/               # connection pool, migration runner
│   │   ├── handlers/
│   │   │   ├── public/       # read-only endpoints, no auth
│   │   │   ├── admin/        # CRUD + publish workflow, auth + RBAC required
│   │   │   └── auth/          # login/refresh/logout
│   │   ├── middleware/       # RequireAuth, RequireRole, rate limiting
│   │   ├── models/           # structs matching DB tables
│   │   └── repository/       # SQL queries
│   ├── migrations/            # numbered up/down SQL files
│   └── .env.example
├── frontend-public/           # placeholder — not yet built
├── frontend-admin/            # placeholder — not yet built
└── docker-compose.yml         # local Postgres, if not using a hosted DB
```

## Installation

Requires Go 1.22+ and a PostgreSQL database (local or hosted).

```bash
cd backend
cp .env.example .env
```

## Environment variables

Set these in `backend/.env` (never commit this file):

| Variable | Description |
|---|---|
| `APP_ENV` | `development` or `production` |
| `PORT` | port the API listens on (default `8080`) |
| `DATABASE_URL` | Postgres connection string, e.g. `postgres://user:pass@host:5432/db?sslmode=require` |
| `JWT_ACCESS_SECRET` | random string, 32+ chars — generate with `openssl rand -base64 48` |
| `JWT_REFRESH_SECRET` | a *different* random string, same requirements |
| `MEDIA_STORAGE_PATH` | local path for uploaded media (default `./media`) |
| `ALLOWED_ORIGINS` | comma-separated list of frontend origins allowed to call the API |

## Database setup

Any Postgres 14+ works. Two options:

**Hosted (recommended if you don't have a local Postgres):** create a free
project at [neon.tech](https://neon.tech), copy the connection string with
**connection pooling turned off** (migrations need a direct session
connection), and paste it into `DATABASE_URL`.

**Local via Docker:** `docker compose up -d postgres` from the repo root
starts Postgres with credentials matching `.env.example` already.

Migrations run automatically every time the server starts — no separate
migrate command needed. They're idempotent, so this is safe on every boot.

## Running locally

```bash
cd backend
go mod tidy
go build ./...        # confirms everything compiles
go run ./cmd/server    # applies migrations, starts listening on :8080
```

Confirm it's up: `curl http://localhost:8080/api/health` → `{"status":"ok"}`

## Creating the first admin

Nobody can create the first admin through the API (creating an admin
requires already being one), so there's a dedicated CLI:

```bash
go run ./cmd/createadmin -email you@example.com -name "Your Name" -role super_admin
```

It prompts for a password with hidden input rather than a flag, so it never
ends up in shell history. Available roles: `super_admin`, `palace_editor`,
`palace_publisher`, `cultural_editor`.

## Running tests

Not yet written. Planned coverage: auth, RBAC, decree/announcement status
transitions, audit logging, and the DB triggers that block silent edits to
published content.

## Building for production

```bash
cd backend
CGO_ENABLED=0 go build -o ochidoma-server ./cmd/server
```

Produces a single static binary — copy it plus the `migrations/` folder to
your server. A `Dockerfile` is also included if you'd rather deploy as a
container.

## Deployment notes

- Set `APP_ENV=production` — this switches refresh-token cookies to
  `Secure` (HTTPS-only).
- Put the API behind HTTPS (via your host's load balancer, a reverse proxy
  like Caddy/nginx, or your PaaS's built-in TLS).
- `DATABASE_URL`, `JWT_ACCESS_SECRET`, and `JWT_REFRESH_SECRET` should be set
  as real environment variables on the host, never committed.
- The same Neon database used in development can be pointed at from
  production — no separate database setup needed unless you want isolated
  dev/prod data.

## Security notes

- Passwords hashed with Argon2id, never stored or logged in plaintext.
- Access tokens are short-lived (15 min); refresh tokens are stored
  server-side as SHA-256 hashes (never plaintext) and rotated on every use.
- `/api/auth/login` is rate-limited to 10 attempts/minute per IP.
- Published decrees and announcements are protected at the database level
  (Postgres triggers reject `UPDATE`s that change published content), in
  addition to the application-level workflow rules — a second line of
  defense if application code ever has a bug.
- Every admin mutation writes an audit log entry (`audit_logs` table),
  including who did it, what changed, and from what IP.

## Admin usage: the publish workflow

Decrees and announcements follow the same state machine:

```
draft → submit → in_review → approve → approved → publish → published → archive → archived
```

- `palace_editor` and `super_admin` can create, edit drafts, and submit for review.
- Only `palace_publisher` and `super_admin` can approve, publish, or archive.
- Once published, content is immutable — corrections create a **new version**
  (decrees) rather than editing the original, which stays visible and intact.
- `cultural_editor` and `super_admin` manage articles/foods/events directly,
  including publishing — these aren't official Palace decrees, so they don't
  require `palace_publisher` sign-off.

### Admin API quick reference

All `/api/admin/*` routes require `Authorization: Bearer <access_token>`
from `/api/auth/login`.

```
POST /api/admin/decrees                          create (draft v1.0)
POST /api/admin/decrees/{id}/submit               draft -> in_review
POST /api/admin/decrees/{id}/approve              in_review -> approved   [publisher/super_admin]
POST /api/admin/decrees/{id}/publish              approved -> published  [publisher/super_admin]
POST /api/admin/decrees/{id}/archive              -> archived            [publisher/super_admin]
POST /api/admin/decrees/{id}/correct              published -> new draft version

POST /api/admin/announcements                     create (draft)
POST /api/admin/announcements/{id}/submit         draft -> in_review
POST /api/admin/announcements/{id}/approve        in_review -> approved  [publisher/super_admin]
POST /api/admin/announcements/{id}/publish        approved -> published [publisher/super_admin]
POST /api/admin/announcements/{id}/archive        -> archived           [publisher/super_admin]

GET/POST/PUT/DELETE /api/admin/articles           [cultural_editor/super_admin]
GET/POST/PUT/DELETE /api/admin/foods              [cultural_editor/super_admin]
GET/POST/PUT/DELETE /api/admin/events             [cultural_editor/super_admin]
```

Public (no auth) equivalents exist at `/api/decrees`, `/api/announcements`,
`/api/articles`, `/api/foods`, `/api/events` — these only ever return
`published` content.
