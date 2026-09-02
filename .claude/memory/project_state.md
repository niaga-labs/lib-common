---
name: project_state
description: The resume point for lib-common — current checkpoint (sha, environment, open units, recommended next unit) at the top, earlier checkpoints below. Read first in every session; rewritten by /recap.
metadata:
  type: project
---

## 2026-09-03 state (resume here)

- **Repo:** `main` @ `03d9096`. Module path is `github.com/niaga-labs/lib-common` since NIAGA-103 (2026-09-03); the
  `replace` for `lib-common` still points at the local `../lib-common`.
- **Environment:** not re-verified today — Docker Desktop was down, so nothing was run against the stack.
  The last stack-verified checkpoint is below. `go build ./...` and `go vet ./...` were clean here on 2026-09-03.
- **Open units** — checked against Jira on 2026-09-03; every ticket the previous block listed had already
  been closed.

| Ticket | State | Blocked on | Note |
|---|---|---|---|
| NIAGA-69 | To Do | **owner decision** | CI cannot resolve this module from a service repo. The fix is written here on `feat/DMB-19-reusable-ci` (31c68e5, **unpushed**); merging it needs a call on how CI reads a private repo — an org PAT secret, or making this repo public |
| NIAGA-117 | To Do | — | subject catalog: marketplace should publish sync-completed/failed on the canonical subjects; two unused subjects are removed or documented as reserved |
| NIAGA-76 | To Do | owner | Windows blocks the `eventsourcing` test binary after the rename |
| NIAGA-151 | To Do | — | deep review of this repo and `service-auth` |

- **Waiting on Luqman:** nothing specific to this repo. One decision blocks CI everywhere: how a workflow
  reads the private `lib-common` (an org PAT secret, or making it public) — NIAGA-69, and the owner-step
  table in the workspace `.claude/memory/standing-decisions.md`.

## Earlier checkpoints

### 2026-08-31

- **Repo:** `main` — the Claude layer was added by NIAGA-17; no code change came with it.
- **Environment:** verified against the shared dev-infra stack on 2026-08-31. Postgres `niaga_db` loads from
  `infra-database/scripts/load_schema.sh --drop` and seeds from `database/seed_local.sh --demo`.
  Builds; consumed by the services.
- **Known gaps in this repo:** no CHANGELOG.
- **Open units**

| Unit / ticket | State | Blocked on | Note |
|---|---|---|---|
| — | | | no open ticket names this repo |

- **Recommended next unit:** NIAGA-18 — add the CHANGELOG this repo still lacks.
- **Waiting on Luqman:** NIAGA-9 (batik-specific vs generic platform) shapes everything downstream; see
  `../docs/REBRAND_INVENTORY.md`.
