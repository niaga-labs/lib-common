---
name: project_state
description: The resume point for lib-common — current checkpoint (sha, environment, open units, recommended next unit) at the top, earlier checkpoints below. Read first in every session; rewritten by /recap.
metadata:
  type: project
---

## 2026-09-06 state (resume here)

- **Repo:** `main`, branch `feat/NIAGA-123-back-in-stock-subject`. Public since 2026-09-05.
- **This unit is ONE THIRD of NIAGA-123**, and the order matters: lib-common declares
  `events.customer.back_in_stock`, **service-customer** then publishes it through an outbox it does not yet
  have, and **service-notification** consumes it with a template. Merge libs → services.
- **Verified:** `gofmt` clean on both files touched, `go build ./...` and `go vet ./...` clean,
  `go test ./... -count=1` **86 pass / 0 fail** (was 81), 5 new.
- **IF YOU ARE PICKING THIS UP MID-CHAIN**, the README table row is marked *(NIAGA-123, landing)* on both
  the publisher and consumer columns. That marker is the state: it means declared, not yet wired at either
  end. Delete the parentheses when both halves are merged. A subject stuck between declared and published is
  precisely how the four orphaned `events.cart.*` entries happened.
- **The subject is DOWNSTREAM of `events.inventory.product.restocked`, not a duplicate.** Inventory says a
  product came back; this says a named customer asked to be told. The subscription lookup between them is
  service-customer's job, so notification never learns what a subscription is.
- **The payload is deliberately self-contained** — customer, product, variant, quantity — so the consumer
  never queries another service. A consumer that looks things up fails when the other service is down.
- **What service-customer needs next, measured:** it has a `*gorm.DB` and NATS wiring but **no outbox and no
  publisher at all** — only `internal/events/subscriber.go`. The no-op to replace is
  `SimpleNotificationClient.SendBackInStockNotification` (subscriber.go:327), whose own TODO proposes an HTTP
  call to service-notification; the ticket forbids that, and correctly — notification has no API. The restock
  path at subscriber.go:255-285 already assembles the full payload, so the change is swapping the no-op for
  an enqueue. Model the publisher on service-support's `internal/events/publisher.go`.
- **What service-notification needs, measured:** it is declarative — one line in `notificationBindings`
  (subscriber.go:29), a `template_router.go` entry and a handler. 18 subjects are routed there today.
- **Recommended next unit:** finish NIAGA-123 — service-customer, then service-notification, then the
  end-to-end: restock a seeded product on the stack and watch Mailpit (localhost:8025).

## 2026-09-06 state (NIAGA-117, superseded above)

- **Repo:** `main`, branch `feat/NIAGA-117-subject-catalog`. Module `github.com/niaga-labs/lib-common`,
  **public** since 2026-09-05.
- **This unit (NIAGA-117): the subject catalog is documented and audited.** New `README.md` (the repo had
  none) carrying the subject > publisher > consumers table; `catalog.go` annotated at the declarations.
- **Verified:** `gofmt` clean on `catalog.go`, `go build ./...` and `go vet ./...` clean,
  `go test ./... -count=1` **81 pass / 0 fail** across 6 packages. service-marketplace re-checked and still
  **162 pass / 0 fail** — it needed no change (see below).
- **HALF THIS TICKET WAS ALREADY DONE AND THE TICKET DID NOT KNOW.** It asks that marketplace publish on
  `SubjectMarketplaceSyncCompleted/Failed` "instead of the bare marketplace.sync.completed
  (internal/events/subscriber.go:15-16)". service-marketplace's `internal/events/publisher.go:18-19` already
  aliases the lib-common constants, and there is no bare subject anywhere in that repo. The cited lines are
  now an import block. Same stale-premise shape as NIAGA-202 and NIAGA-116.
- **ITS GATE HAD BEEN MET FOR TWO DAYS AND NOBODY RE-CHECKED.** The 2026-09-03 skip comment said the gate
  ("the canonical-subjects ticket is Done") was unmet because NIAGA-116 was To Do. NIAGA-116 went Done on
  2026-09-04. The skip was right when written and stale by the next morning — worth remembering that a
  gate comment is a snapshot, not a standing fact.
- **The audit used two methods, and that is the part worth keeping.** Constant search plus literal-string
  search across every tracked file of every repo. `~/.claude/rules/verification.md` requires a second method
  before acting on a "no matches", and this ticket's whole output is a set of no-matches.
- **Four categories, not the two the ticket names:** publisher+consumer (20 subjects) · publisher, no
  consumer (the 2 marketplace sync) · Reserved, neither (2) · routed by service-notification but absent from
  the catalog entirely (4, the cart/email-verification set).
- **Recommended next unit:** **NIAGA-183** — `marketplace.platform_credentials` is wired into `main.go` and
  `routes.go` but its table was never added to `infra-database`, so two endpoints 500 and two of the six
  bruno probes fail. **The dev stack is UP** as of this session, so needs-stack work is viable again.
  Otherwise NIAGA-54 piece 3c (`UpdateOrderStatus`), filed as its own key the way 205 was.

## 2026-09-03 state (superseded above)

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
