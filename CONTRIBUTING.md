# Contributing to MINE

## Setup

```bash
cp .env.example .env
./scripts/setup-dev.sh
docker compose up --build
```

Requires: Go 1.22+, Node 20+, Docker 24+, golang-migrate, make.

## Quality gates (run before pushing)

```bash
cd panel/backend && go vet ./... && go test ./... -race
cd daemon && go vet ./... && go test ./... -race
cd panel/frontend && npm run lint && npm run build
```

## Conventions

- Go: `gofmt` + `go vet` clean, errors wrapped with `%w`, no `panic` in request paths.
- Frontend: TypeScript strict, functional components, Tailwind for styling.
- Migrations: `golang-migrate`, `migrations/NNNN_name.up.sql` + `.down.sql`, never edit applied migrations.
- Templates: validate YAML against schema in `templates/README.md`.
- Never commit secrets. Never force-push.

## PRs

Keep PRs small, add/extend tests, update `docs/` when changing APIs or protocol.
