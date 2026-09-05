# Changelog

All notable changes to `lib-common` are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added — events.customer.back_in_stock (NIAGA-123)

- **The subject, its payload and its tests**, so the two repos that publish and consume it have something to
  build against. This is the `lib-common` third of NIAGA-123; service-customer publishes it and
  service-notification consumes it, in that order.
- **It is downstream of `events.inventory.product.restocked`, not a duplicate.** Inventory says a product came
  back; this says a *named customer asked to be told*. The subscription lookup between the two belongs to
  service-customer, so notification never has to know what a subscription is.
- **`CustomerBackInStockPayload` carries everything the email needs** — customer, product, variant, quantity —
  so the consumer never queries another service. A notification consumer that has to look things up fails
  when the other service is down, and produces an email nobody can explain afterwards. The publisher already
  has all of it in hand at the moment it matches the subscription.
- **The wire shape is pinned, not the Go struct** (`~/.claude/rules/go.md`). `TestBackInStockEmitsEveryRequiredKey`
  marshals a populated payload and checks the keys the template depends on; `omitempty` on a required field
  is the bug, because a consumer cannot tell *absent* from *empty*.
- **A zero `stock_quantity` survives.** Zero is meaningful here — the restock that triggered the email has
  already sold out again — and is different from no quantity at all. `omitempty` on that int would erase
  exactly that case.
- **The variant fields really are omitted when absent**, rather than emitted empty: an empty string renders as
  a blank line in a template instead of being skipped by it.
- **`TestNewSubjectsAreRegisteredInSubjectDomains`** closes a gap NIAGA-117 documented but did not guard: a
  subject declared and left out of `SubjectDomains` is invisible to anything walking the catalog. Go cannot
  enumerate a package's constants, so it checks the recent subjects rather than claiming to be exhaustive —
  the README table remains the exhaustive record.
- The README table gains the row, marked **landing** rather than Reserved: Reserved means nobody intends to
  build it; this one is mid-build, and the note says to delete the marker once NIAGA-123 closes. A subject
  stuck between *declared* and *published* is exactly how the four orphaned `events.cart.*` entries happened.
- Tests: **86 pass, 0 fail** (was 81) — 5 new. `go build ./...` and `go vet ./...` clean.


### Added — README with the subject > publisher > consumers table (NIAGA-117)

- **`lib-common` had no README at all.** The ticket's done-when is "the README table is complete"; there was
  no README to complete. One now exists, with the event subject catalog as its centrepiece plus a package
  index, because a public module with no README is its own gap (public since 2026-09-05).
- **Every one of the 24 catalog subjects now has a publisher and a consumer, or says here why not.**
  Measured 2026-09-06 by **two independent methods** — a search for `eventsourcing.<Const>` across every Go
  file in the workspace, and a literal search for each subject *string* in every tracked file of every repo,
  frontends included. The two agree.
- **Three rows are not a simple pair, and each is now written down rather than left to be re-derived:**
  - `events.inventory.stock.updated` has **two** publishers — service-inventory *and* service-order, both
    correctly through the outbox. Not to be confused with NIAGA-178, a separate bare-subject publish in
    service-order that bypasses the outbox and reaches nobody.
  - The two `events.marketplace.sync.*` subjects have a publisher and **no consumer**. Real events on the
    wire that nothing subscribes to — not a defect, and not the same as Reserved.
  - `events.customer.created` and `events.agent.commission.paid` are **Reserved**: declared, never published,
    never consumed, appearing nowhere in the workspace but `catalog.go`.
- **The two Reserved subjects were KEPT, not removed** — the ticket allowed either. Nothing imports them so
  deletion would be safe, but they name planned work (service-customer exists; service-agent is
  legacy-hidden rather than deleted) and the constant is the only surviving record of that intent. Deleting
  costs that and saves nothing; keeping it cheap to remove later.
- **A fourth category the ticket did not name is documented too:** four subjects are routed and handled in
  `service-notification/internal/events/template_router.go` with **no catalog entry, no publisher and no
  consumer** — `events.user.email_verification_requested` and the three `events.cart.abandoned*`. A reader
  would reasonably conclude cart-abandonment email works. It cannot.
- `catalog.go` carries the same facts at the declarations, so someone reading the constants sees them
  without opening the README.
- **A count in the first draft was wrong and review caught it.** The README said lib-common is "consumed by
   the eight Go services". It is **ten** — every `service-*` repo carries the `replace` directive. The "eight"
   is the number of services with a **CI workflow** (`service-marketplace` and `service-support` have none),
   which is the phrasing the workspace's `ci-known-red.txt` uses. Two different sets, one number; both are
   now stated so the conflation does not recur. Fitting error for a document whose premise is "measured, not
   assumed" — it was the one line in it that wasn't.
- **A second stale fact of my own, found while checking the review's reasoning.** The first draft described
  **NIAGA-178** in the present tense — "a bare-subject publish in service-order that bypasses the outbox and
  reaches nobody". It is **fixed and Done**: no bare publish remains anywhere in service-order, and the code
  carries a comment saying what it used to do. Corrected to the past tense, and kept in the README rather
  than dropped, because "service-order publishes stock.updated" is true both before and after the fix — only
  the subject told them apart.
- **This repo's own `CLAUDE.md` was stale in three places** and is corrected here, since the parent epic is
  literally *Docs tell the truth*: it said `CHANGELOG.md` "does not exist yet" (it does), listed **NIAGA-69**
  as blocked on an owner decision (Done — this repo went public on 2026-09-05), and counted **6** test files
  (there are **10**).
- Tests: **81 pass, 0 fail** in this repo (unchanged — this is documentation and comments).
  `go build ./...` and `go vet ./...` clean.

### Added — auth.NewInternalHTTPClient, the calling half of the guard (NIAGA-114)

- The service clients that call `/internal/*` build requests in a dozen places each, mostly through
  `http.Client.Post`, with no shared helper to hang a header on. `InternalTokenTransport` sets
  `X-Internal-Token` in a `RoundTripper` instead, so it covers **every** request a client makes — including
  the ones someone adds later without reading the ticket.
- An empty token sends **no** header, so a misconfigured caller takes a clear 401 from the callee rather than
  presenting an empty credential.
- `RoundTrip` clones the request rather than mutating the caller's, as the `http.RoundTripper` contract
  requires.
- 4 tests, including one that runs the client against a server applying the same rule the middleware does —
  the two halves have to agree or the guard passes nothing.

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
