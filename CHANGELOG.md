# Changelog

All notable changes to `lib-common` are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added — auth.InternalToken, a guard for the service-to-service routes (NIAGA-114)

- The `/api/v1/internal/*` routes across four services reserve, deduct and restock inventory, reserve
  flash-sale allocations, approve agent commissions and create marketplace orders. **Not one of them checked
  anything.** `service-inventory`'s block carried the comment *"should be protected by internal
  network/service mesh in production"* and `service-order`'s said *"No auth required - called by marketplace
  service"*. There is no service mesh, and nginx proxies those paths.
- `auth.InternalToken(expected)` requires an `X-Internal-Token` header matching in **constant time**
  (`crypto/subtle`). `auth.ResolveInternalToken(appEnv)` reads `INTERNAL_API_TOKEN`, falls back to the
  published `dev-internal-token` placeholder **in development only**, and returns an error outside it — both
  when unset *and* when it still holds the placeholder, which is in every `.env.example` in the workspace.
  `MustResolveInternalToken` panics, for a `main()` that should not start without one.
- **An unconfigured service authenticates nobody.** An empty expected token refuses every request, including
  an empty header. If it accepted anything when unconfigured, a service that failed to read its environment
  would silently serve stock movements to the world — the shape of NIAGA-170.
- `IsDevEnv` treats an unrecognised `APP_ENV` as production, because guessing wrong in that direction is the
  safe way to be wrong.
- 9 tests: missing, wrong, correct and prefix tokens; the unconfigured case; the dev fallback; the placeholder
  refused outside development; whitespace trimmed.
- Deliberately **not** the database-backed `APIKeyMiddleware` already in this package, which no service uses.
  Callers here are our own services on a private network, and a shared token read from the environment is the
  smallest thing that closes the hole. Per-service keys with scopes remain a later question.

### Added — the order event payload contract (NIAGA-166)

- `eventsourcing/order_payloads.go` defines `OrderCreatedPayload`, `OrderStatusChangedPayload`,
  `OrderEventItem` and `OrderShippingAddress` — the contract for the two order subjects that carry
  customer-facing notifications. It lives here because it is an agreement between two services that never
  import each other, and until now there was nowhere for it to live, so the two sides drifted in **five**
  separate ways at once and every one of them was silent.
- The drift, for the record: the publisher sent `new_status` and the consumer read `status`; `customer_email`
  and `customer_name` were never published; the address was `address` as an object against `shipping_address`
  as a string; items were `{product_id, unit_price}` against `{name, price}`; and the SMS leg was handed an
  email address. `encoding/json` does not error on a field the sender omitted, so nothing failed, nothing
  retried, and nothing reached a dead-letter queue — the order moved to shipped and the customer heard
  nothing.
- `DeliverableTo()` on both payloads, so a consumer can skip and say so instead of calling an email service
  with an empty `To` — which is what `service-notification` did. `OrderShippingAddress.OneLine()` renders an
  address for a mail or SMS body, skipping empty parts.
- 6 tests pin the **wire format**, not the Go structs: every required JSON key must be emitted by a populated
  payload, `status` must not appear where `new_status` is meant, and a round trip must keep the recipient, the
  product names, the unit prices and the phone number. They fail on a rename, which is the change that breaks
  a consumer silently.

### Changed - the Claude layer says Niaga and lists only open units (NIAGA-105)

- `CLAUDE.md` and `.claude/memory/MEMORY.md` are titled **Niaga**, not Desa Murni Batik, and the Jira project
  is **NIAGA**. Ticket keys in both files moved from `DMB-n` to `NIAGA-n` (same issues, the old keys are
  aliases); git branch names keep the `DMB-` they were created with, because those are real refs.
- **The open-units table was wrong, not just stale**: every ticket it listed had already been closed. It now
  lists the tickets that actually name this repo, checked against Jira on 2026-09-03.
- `.claude/memory/project_state.md` gained a `2026-09-03` resume block with the current default branch and
  sha; the previous block was demoted to a checkpoint unedited, so it still reads as what was true that day.

### Changed — module path moved to github.com/niaga-labs (NIAGA-103)

- The GitHub org was renamed `KilangDesaMurniBatik` → **`niaga-labs`**, so the module is now
  **`github.com/niaga-labs/lib-common`**. `go.mod`, the three internal imports in `middleware/` and the
  `CLAUDE.md` header moved with it; the remote was repointed. Consumers change their `require` line and the
  local `replace` target name — the `=> ../lib-common` path itself is unchanged.
- History lines in this file keep the old path on purpose: they record what was true then.

### Fixed — InjectTracingHeaders passed a lock by value (DMB-93)

- `telemetry.InjectTracingHeaders` took `gin.Context` **by value**. `gin.Context` contains a `sync.RWMutex`,
  so every call would have copied a lock — `go vet`: *passes lock by value*. A copied mutex guards nothing,
  because the copy has its own.
- Now takes `*gin.Context`. Every gin handler already holds a pointer, so there was never a reason to take a
  value. Nil-guards `ctx`, `ctx.Request` and `req` while in there.
- **This is a signature change on an exported function**, but nothing calls it — checked every service and
  library in the workspace. It was an unused helper with a latent bug rather than a working one.
- With this, **`go vet ./...` is clean in all eleven Go repos**. It was not before.

### Fixed — empty collections serialised as null (DMB-74)

- **`"data": null` where a list was promised.** `Success`, `SuccessWithMeta` and `SuccessWithPagination` passed
  the handler's value straight to `c.JSON`. A Go repository with no rows returns a **nil slice**, and
  `encoding/json` writes a nil slice as `null` — so `GET /api/v1/inventory/movements` and
  `GET /api/v1/admin/payments` answered `{"success":true,"data":null,...}` on an empty result. Every client
  had to guard for null before calling `.map()`, and two bruno probes had to drop their `data: isArray`
  assertion because of it.
- A nil slice or nil map now becomes an empty one before it is written, so an empty list is `[]` and an empty
  map is `{}`. Fixed here rather than in each handler: all ten services reach `c.JSON` through this package,
  so one change covers every paginated and list endpoint instead of a sweep that would miss the next one.
- Deliberately untouched, each pinned by a test:
  - an **untyped nil** — `Deleted` and friends still answer with no data;
  - a **nil pointer** — "the object you asked for does not exist" is genuinely `null`, and turning it into
    `[]` would be a lie;
  - any non-nil collection, including one that is already empty.

### Added — tests (DMB-74)

- `response/response_test.go`, 6 passing — first tests for this package. They assert on the decoded JSON a
  client actually receives, not on Go values: nil slice through `Paginated` and through `OK`, nil map, a
  populated slice passing through unchanged, a nil pointer staying `null`, and `Deleted` unaffected.

### Changed

- **Module path moved to the surviving org.** `github.com/niaga-platform/lib-common` is now
  `github.com/KilangDesaMurniBatik/lib-common`, along with every import that named the old org — 3 Go
  files here. Two GitHub orgs held the same product; `niaga-platform` was last touched in
  December 2025 and is retired, so `KilangDesaMurniBatik` is the only one. Nothing outside `.go` referenced
  the old path, so this is a pure import rename: no behaviour change, no dependency change. The
  `replace` directive still points at `../lib-common`; how services depend on it is DMB-72. (DMB-71)

### Notes

- **`eventsourcing` could not be run on the Windows dev laptop after the rename.** `go build` and
  `go vet` are clean and the test binary compiles, but Windows Application Control refuses to launch
  it: *"An Application Control policy has blocked this file"*, and running the compiled `.exe`
  directly gives `Permission denied`. It is specific to that one binary and reproducible — the other
  three packages rebuild and pass, the same package passes on the pre-rename source, and renaming the
  output file does not help, so it is a content-keyed reputation false positive rather than anything
  about the test. Nothing but an import path string changed, and the block happens at `fork/exec`
  before the test starts, so no test logic is involved. Linux CI is unaffected. Tracked in DMB-76.
- Shared Go library consumed by every service as a module dependency; it has no `cmd/` and no port.
- 6 `*_test.go` files — the only repo besides `service-marketplace` with any tests at all.
- Owns `events` and `outbox` in `niaga_db` (the transactional outbox the services publish through).
