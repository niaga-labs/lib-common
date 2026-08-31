# Desa Murni Batik — lib-common

Shared Go library for every Desa Murni service: config loading, database and NATS helpers, auth middleware, logging, the transactional outbox and event sourcing.
Jira project **DMB** · GitHub `KilangDesaMurniBatik/lib-common` · Go 1.24.0 · module `github.com/KilangDesaMurniBatik/lib-common`.
Library, not a service: no `cmd/`, no port, no database of its own.
Owns `events`, `outbox` in `niaga_db` (3 tables referenced by `TableName()`).

Global rules live in `~/.claude/`; this file only adds what is specific here.

## Orient here first

- `.claude/memory/project_state.md` — **resume here** (`/continue` reads it, `/recap` rewrites it).
- `CHANGELOG.md` — what changed (**does not exist yet** — create on first ship).
- `../CLAUDE.md` — the workspace repo map and the cross-repo change order.
- `../infra-platform/docs/LOCAL_DEV.md` — bringing the whole platform up locally.

## Commands

| Task | Command |
|---|---|
| install | `go mod download` |
| build | `go build ./...` |
| test | `go test ./...` |
| lint | `gofmt -l . && go vet ./...` |


## Conventions that differ from the global rules

- **`infra-database/schemas/` owns the schema, not this repo.** A schema change is a migration there plus a
  re-export, never an `AutoMigrate` and never a hand edit of a dumped schema file.
- Default branch is `main`.
- Protected paths (never edited in place, see `.claude/protected-paths.txt`): `migrations/*.sql`.

## Where things are

- Entry point: no `cmd/` — this is a library
- Packages: `auth`, `config`, `database`, `domain`, `eventsourcing`, `lock`, `logger`, `middleware`, `monitoring`, `nats`
- Config: `config/`
- Tests: 6 `*_test.go` file(s)

## Open units

| Ticket | State | Blocked on | Note |
|---|---|---|---|
| — | | | no open ticket names this repo |
