# ⛏️ MINE — Next-gen game server hosting panel

> A modern, blazing-fast alternative to Pterodactyl. Panel + Daemon for Minecraft & game servers, reimagined with great UX.

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)
[![Stack: Go + Vite](https://img.shields.io/badge/Stack-Go%20%2B%20Vite-blue.svg)](#tech-stack)
[![Docker Compose](https://img.shields.io/badge/Deploy-Docker_Compose-2496ED.svg)](#quickstart)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

**MINE** is an open-source game server management platform — think Pterodactyl, but better: modern UI, lighter daemon, real-time everything, and multi-game from day one.

## Why MINE? (vs Pterodactyl)

| Feature | Pterodactyl | MINE |
|---|---|---|
| UI / UX | Dated, heavy, desktop-only | Modern, fast, mobile-first UI with dark mode |
| Real-time console | Polling / laggy websockets | Live WebSocket console, stats, logs with low latency |
| Daemon (Wings) | Heavy on low-end nodes | Lightweight Go daemon, low RAM/CPU footprint |
| Multi-game | Mostly Minecraft-centric eggs | First-class templates: Minecraft Java + Bedrock, Rust, CS2/CSGO… |
| Installation | Manual, error-prone | One-command Docker Compose stack |
| API | Limited, inconsistent | Clean REST + WebSocket API (Fiber), typed shared protocol |
| Backups | Local / S3 bolt-on | S3-compatible built in (MinIO in dev, any S3 in prod) |

Goal: **same power as Pterodactyl, none of the pain.**

## ✨ Features

- 🎮 **One-click servers** — Minecraft Java (Paper/Purpur/Forge/Fabric/Vanilla), Bedrock, proxies (Velocity/BungeeCord), Rust, CSGO/CS2 (see `templates/`)
- 🖥️ **Modern dashboard** — server list, live stats (CPU/RAM/disk/players), file manager, subusers, backups, databases
- ⚡ **Live console** — WebSocket-powered terminal, command history, player actions
- 📁 **Web file manager + SFTP** — browser editor, zip/unzip, drag & drop upload
- 🕹️ **Version manager** — switch JAR/version easily, schedulers & auto-restart
- 🧩 **Template system** — extensible Docker-image “eggs” under `templates/` for any game
- 👥 **Users & roles** — auth with JWT + 2FA (TOTP), subusers with granular permissions
- 📊 **Node monitoring** — health, capacity, allocation management, daemon ↔ panel WS heartbeat
- 🔌 **API-first** — REST + WS, shared Go protocol in `shared/`

## 🏗️ Architecture

```
┌──────────┐   :3000   ┌────────────┐   :8080   ┌──────────────┐  WS/gRPC  ┌─────────────┐
│ Browser  │ ◄────────► │  Frontend  │ ◄────────► │ Panel (Go +  │ ◄────────► │ Daemon (Go) │
│          │           │ (Vite SPA) │  REST/WS  │ Fiber API)   │           │  per node   │
└──────────┘           └────────────┘           └──────┬───────┘           └──────┬──────┘
                                                      │ PG + Redis + S3          │ Docker
                                               ┌──────▼───────┐           ┌──────▼──────┐
                                               │ Postgres/    │           │ Game server │
                                               │ Redis/MinIO  │           │ containers  │
                                               └──────────────┘           └─────────────┘
```

Services (dev `docker-compose.yml`):

| Service | Port | Description |
|---|---|---|
| `frontend` | 3000 → 80 | Vite SPA, talks to panel via `VITE_API_URL` |
| `panel` | 8080 | Go/Fiber API, serves auth/servers/nodes, daemon hub |
| `daemon` | 8081 | Go daemon, Docker lifecycle, console/stats/filesystem WS |
| `postgres` | 5432 | Primary DB (GORM + migrations) |
| `redis` | 6379 | Sessions, queue, pub/sub |
| `minio` | 9000/9001 | S3-compatible backup storage |

## 🛠️ Tech Stack

- **Panel backend:** Go 1.22, Fiber v2, GORM (Postgres), go-redis, JWT + TOTP 2FA, `golang-migrate`
- **Daemon (`mined`):** Go, Docker API, filesystem/SFTP, console + stats streaming over WebSocket
- **Frontend:** Vite + TypeScript + Tailwind (SPA in `panel/frontend/src`)
- **Shared:** Go module `shared/` (proto + types for panel↔daemon contract)
- **Infra:** PostgreSQL 16, Redis 7, MinIO (S3), Caddy/Traefik + auto-HTTPS in prod
- **Tooling:** Docker Compose, Make, `go vet`, `go test -race`

## 📁 Repository layout

```
MINE/
├── panel/backend/      # Go/Fiber API (auth, servers, nodes, daemon hub, migrations)
│   ├── cmd/mine/       # panel entrypoint
│   ├── internal/{api,auth,config,daemon,middleware,models,repository,services}/
│   └── migrations/     # golang-migrate SQL (up/down, never edit applied ones)
├── panel/frontend/     # Vite + TS + Tailwind SPA (src/{components,pages,hooks,lib,services})
├── daemon/             # Go daemon `mined` (cmd/mined, internal/{config,console,docker,filesystem,server,stats,websocket})
├── shared/             # shared Go protocol (proto + types)
├── templates/          # server templates: minecraft-java, minecraft-bedrock, rust, csgo
├── tests/{e2e,integration}/  # end-to-end + integration suites
├── scripts/            # e.g. setup-dev.sh (secrets/TLS for dev)
├── docs/               # extra docs (API, protocol changes go here)
├── docker-compose.yml  # full dev stack
├── docker-compose.prod.yml  # prod images from GHCR + secrets
├── Makefile            # build/test/lint/migrate/up/down
└── .env.example        # copy to .env to configure
```

## 🚀 Quickstart

Requirements: Docker 24+, Go 1.22+, Node 20+, `make`, `golang-migrate`.

```bash
git clone https://github.com/zema-tech/MINE.git
cd MINE
cp .env.example .env
./scripts/setup-dev.sh
docker compose up --build
```

Then:

- Frontend → http://localhost:3000
- Panel API → http://localhost:8080
- MinIO console → http://localhost:9001
- Daemon → http://localhost:8081 (registered to panel via token)

Or with Make:

```bash
make up        # docker compose up --build (full dev stack)
make down      # stop + drop volumes
make build     # build panel + daemon binaries locally
make test      # go test ./... -race across panel/backend, daemon, shared
make lint      # go vet across Go modules
```

### ⚙️ Configuration

Copy `.env.example` to `.env` and change secrets. Key vars:

| Var | Default | Purpose |
|---|---|---|
| `POSTGRES_*` | `mine / mine_dev_secret` | Postgres user/password/db |
| `REDIS_PORT` / `REDIS_PASSWORD` | `6379` | Redis (prod requires password) |
| `PANEL_PORT` / `FRONTEND_PORT` / `DAEMON_PORT` | `8080 / 3000 / 8081` | Published ports |
| `DAEMON_TOKEN` | `dev_daemon_token` | Daemon ↔ panel auth (use Docker secret in prod) |
| `JWT_SECRET` / `ENCRYPTION_KEY` | dev placeholders | **Must change** — auth + at-rest encryption |
| `APP_URL` / `APP_HOST` | `localhost:8080` | Public panel URL (prod uses `wss://…`) |
| `MINIO_USER` / `MINIO_PASSWORD` | dev placeholders | S3 backup storage |
| `MINE_ADMIN_EMAIL` / `MINE_ADMIN_PASSWORD` | `admin@example.com` | Bootstrap admin |

Never commit `.env` or real secrets.

## 🗺️ Roadmap

- [x] Monorepo scaffold (panel, daemon, shared, templates, compose)
- [ ] Panel: auth (JWT+2FA), servers/nodes CRUD, daemon hub
- [ ] Daemon MVP: Docker lifecycle + WS console + stats + filesystem
- [ ] File manager + SFTP + backups to S3
- [ ] Minecraft template pack (Paper/Purpur/Forge/Fabric/Velocity)
- [ ] Schedulers, auto-restart, subusers/permissions
- [ ] Multi-game templates (Rust, FiveM, CS2, Palworld)
- [ ] Billing/marketplace hooks + Pterodactyl importer
- [ ] v1.0 + one-line installer

Have a priority? [Open an issue](https://github.com/zema-tech/MINE/issues).

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Quick version:

1. Fork → feature branch → PR (keep it small, add tests)
2. Conventional Commits (`feat:`, `fix:`, …)
3. Before pushing: `go vet` + `go test -race` (backend/daemon/shared), `npm run lint && npm run build` (frontend)
4. Migrations: new `NNNN_name.up.sql` + `.down.sql`, never edit applied ones; update `docs/` on API/protocol changes
5. Never commit secrets, never force-push

## 📄 License

AGPL-3.0 — see [LICENSE](LICENSE).

---

<p align="center">Built for hosts who outgrew Pterodactyl. ⛏️</p>
