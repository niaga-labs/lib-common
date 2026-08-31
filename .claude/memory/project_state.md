---
name: project_state
description: The resume point for lib-common — current checkpoint (sha, environment, open units, recommended next unit) at the top, earlier checkpoints below. Read first in every session; rewritten by /recap.
metadata:
  type: project
---

## 2026-08-31 state (resume here)

- **Repo:** `main` — the Claude layer was added by DMB-17; no code change came with it.
- **Environment:** verified against the shared dev-infra stack on 2026-08-31. Postgres `niaga_db` loads from
  `infra-database/scripts/load_schema.sh --drop` and seeds from `database/seed_local.sh --demo`.
  Builds; consumed by the services.
- **Known gaps in this repo:** no CHANGELOG.
- **Open units**

| Unit / ticket | State | Blocked on | Note |
|---|---|---|---|
| — | | | no open ticket names this repo |

- **Recommended next unit:** DMB-18 — add the CHANGELOG this repo still lacks.
- **Waiting on Luqman:** DMB-9 (batik-specific vs generic platform) shapes everything downstream; see
  `../docs/REBRAND_INVENTORY.md`.

## Earlier checkpoints

(none yet)
