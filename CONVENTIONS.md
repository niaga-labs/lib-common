# Niaga API Conventions

Source of truth for the v1 REST API standards every Niaga HTTP service
follows. Lives in `lib-common` because that's the shared library every
service depends on; whenever a service deviates from this doc the bug is
in the service.

When something in this doc and a service's behaviour disagree, **this doc
wins** — the service has a follow-up PR to write.

Drift is enforced through three layers (Phase 9 of the API
standardization plan):

1. Per-service **pre-commit grep guards** (this doc, [§9](#9-pre-commit-grep-guards)).
2. Per-service **OpenAPI specs** at `<service>/docs/openapi.yaml`.
3. Workspace-level **Bruno smoke + envelope assertion** in
   `infra-platform/.github/workflows/api-conformance.yml`.

---

## 1. URL prefix

```
/api/v1/<service>/<resource>[/:id]
```

- One canonical prefix per HTTP service. Services are: `auth`, `catalog`,
  `customer`, `inventory`, `order`, `marketplace`, `agent`, `reporting`,
  `support`. (`notification` is NATS-only and exposes no HTTP routes.)
- Admin-flavored routes nest under the service prefix:
  `/api/v1/<service>/admin/<resource>`.
- Customer / agent self-service routes nest as `/api/v1/<service>/me/...`
  (currently used by `agent`; `customer` uses `/api/v1/customer/*` flat).
- Internal service-to-service routes nest as
  `/api/v1/<service>/internal/...`.

### Webhooks are forever

`/api/v1/webhooks/{shopee,tiktok,parceldaily}` and the Curlec callback at
`/api/v1/payment/webhook` are partner-registered. They cannot be moved
under any service prefix and never sunset. Their response shapes also
break the canonical envelope (see [§3](#3-response-envelope)) — partners
parse the raw payload verbatim.

### Dual-route window

When an existing prefix migrates to its canonical form (Phase 6/7), the
service registers BOTH old and new for one release as a safety net.
nginx adds `Deprecation: true` and `Sunset: <date>` headers on the
legacy `location` blocks. Phase 10 deletes the dual route after
`access.log` shows zero hits for the legacy prefix.

---

## 2. JSON casing

**Snake_case everywhere.**

- Top-level envelope keys: `total_pages`, `total_count`.
- Domain payload keys: `created_at`, `customer_id`, `order_status`.
- Path params: `:id` for the single-resource case, camelCase for
  compounds (`:productId`, `:variantId`, `:orderId`, `:returnId`,
  `:fulfillmentId`, `:itemId`, `:noteId`, `:refundId`, `:jobId`,
  `:mappingId`, `:importedId`).

The frontend consumes camelCase. That conversion happens at the BFF
proxy layer (`frontend-admin/src/app/api/proxy/[...path]/route.ts` and
`frontend-storefront/app/api/proxy/[...path]/route.ts`) — not in any
service. Each service stays snake_case end-to-end.

External partner DTOs (Shopee, TikTok, Parcel Daily, SF Express, Curlec,
Razorpay) keep whatever casing the partner ships. Don't snake_case
those.

---

## 3. Response envelope

Every public API response goes through one of the helpers in
`lib-common/response/response.go`. **Never** `c.JSON(http.StatusXxx,
gin.H{...})` directly.

```jsonc
{
  "success": true,
  "message": "Optional human-readable summary",
  "data":    { /* domain payload */ },
  "error":   null,
  "meta":    { /* paginated responses only */ }
}
```

Failure case:

```jsonc
{
  "success": false,
  "error": {
    "code":    "ORDER_NOT_FOUND",
    "message": "The requested order was not found.",
    "details": null
  }
}
```

Helper menu (full list in [`response/README.md`](response/README.md)):

| Helper                                                 | HTTP |
|--------------------------------------------------------|------|
| `response.OK(c, msg, data)`                            | 200  |
| `response.Created(c, msg, data)`                       | 201  |
| `response.NoContent(c)`                                | 204  |
| `response.Paginated(c, items, page, limit, total)`     | 200  |
| `response.BadRequest(c, msg, details)`                 | 400  |
| `response.Unauthorized(c, msg)`                        | 401  |
| `response.Forbidden(c, msg)`                           | 403  |
| `response.NotFound(c, msg)`                            | 404  |
| `response.Conflict(c, msg)`                            | 409  |
| `response.ValidationError(c, msg, fields)`             | 422  |
| `response.TooManyRequests(c, msg, details)`            | 429  |
| `response.InternalServerError(c, msg)`                 | 500  |
| `response.ServiceUnavailable(c, msg)`                  | 503  |

Webhook handlers are the ONLY exception. Tag them with a comment
pointing to `feedback-webhook-envelope-exception.md` in the workspace.

---

## 4. Pagination

```
GET /api/v1/<service>/<resource>?page=1&limit=20
```

Returned `meta` block:

```jsonc
{
  "page": 1,
  "limit": 20,
  "total_pages": 5,
  "total_count": 87,
  "total": 87        // alias for total_count, frontend convenience
}
```

- `limit` is canonical. The legacy `per_page` and `pageSize` query
  params are accepted for one release as a Phase 4 fallback; new
  clients must use `limit`.
- `offset` is not supported. If a service still has it, that's a bug.

---

## 5. Path params

- Single-resource: `:id` (UUID).
- Compound (resource id + sub-resource): camelCase
  (`:<resource>Id` — `:productId`, `:variantId`, `:orderId`,
  `:itemId`, `:fulfillmentId`, `:refundId`, `:noteId`,
  `:mappingId`, `:importedId`, `:returnId`, `:jobId`,
  `:categoryId`).
- Never snake_case in URLs (`:product_id` is wrong; rename to
  `:productId`).

---

## 6. Auth + middleware chain

Every HTTP service wires lib-common's canonical chain:

```go
libmiddleware.SetupCommonMiddleware(router, libmiddleware.SetupConfig{
    Logger: logger,
    PreRequestID: []gin.HandlerFunc{
        telemetry.TracingMiddleware("service-x"),
        sentryMonitor.GinMiddleware(),
        sentryMonitor.RecoveryMiddleware(),
    },
    AllowedOrigins:        getEnv("ALLOWED_ORIGINS", "..."),
    EnableInputValidation: true,
})
```

This installs (in order): recovery → PreRequestID slot → request-id →
logger → CORS → security-headers → input-validation →
PostValidation slot.

Auth middleware is one of:

- `libmiddleware.AuthMiddleware(jwtManager)` — validates a JWT signed by
  service-auth, sets `user_id` (string) on the context.
- A service-local `AuthMiddleware` that extracts `user_id` as
  `uuid.UUID`. This is the historical shape; services migrating to the
  lib-common middleware add a `userIDFromContext` helper that handles
  both shapes (see service-support and service-order for examples).

Admin gates use `libmiddleware.RequireAdmin()` after the auth middleware.

Request-ID propagation uses the canonical `X-Request-ID` header through
`lib-common/middleware/requestid.go`. Don't introduce competing
correlation IDs (e.g. the deleted `service-inventory`
`X-Correlation-ID`).

---

## 7. Health endpoints

Every HTTP service exposes:

- `GET /health` — liveness, always 200 if the process is up.
- `GET /health/ready` — readiness, 200 when every wired dep is reachable
  and 503 if any dep is degraded.

Wire them through `libmiddleware.RegisterHealth`. Don't roll a
per-service `/health` handler.

---

## 8. OpenAPI

- Each HTTP service ships `<service>/docs/openapi.yaml`.
- OpenAPI 3.0.3, hand-written. No `swag` reflection — stale comments
  lie. No client codegen yet.
- Server URLs: `https://niaga.local` (placeholder gateway) +
  `http://localhost:<port>` (local).
- Document the canonical surface in full. Legacy paths get one
  representative deprecation marker per prefix (`deprecated: true` plus
  a description note).
- Bruno smoke pins the runtime envelope shape; OpenAPI is the reference
  contract. They're allowed to disagree on edge cases — Bruno wins
  because it's executed.

---

## 9. Pre-commit grep guards

Each service repo should install a pre-commit hook that blocks raw
`gin.H{...}` envelope drift before it reaches CI. Two checks:

```bash
#!/usr/bin/env bash
# .git/hooks/pre-commit  (or via husky / pre-commit framework)
set -e

# Block raw response-envelope keys outside the response package itself.
HITS=$(git diff --cached --name-only --diff-filter=ACM \
  | grep -E '^(internal|cmd)/.*\.go$' \
  | xargs -I{} grep -nE 'gin\.H\{"(error|success|data|message)":' {} \
  || true)

if [ -n "$HITS" ]; then
  echo "✗ Raw envelope keys in gin.H{} — use lib-common/response helpers."
  echo "$HITS"
  exit 1
fi

# Block snake_case path params in route registrations.
ROUTES=$(git diff --cached --name-only --diff-filter=ACM \
  | grep -E '^(internal|cmd)/.*\.go$' \
  | xargs -I{} grep -nE '\.(GET|POST|PUT|DELETE|PATCH)\([^)]*"[^"]*:[a-z]+_[a-z_]+' {} \
  || true)

if [ -n "$ROUTES" ]; then
  echo "✗ snake_case path params — convert to camelCase compound (:productId, :orderId, etc.)."
  echo "$ROUTES"
  exit 1
fi
```

Exempt the `lib-common/response/*.go` files themselves from the first
check (they construct the envelope).

To install in a service repo:

```bash
cp scripts/pre-commit-conventions .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

CI in each service repo can run the same check via:

```yaml
# .github/workflows/conventions-check.yml
name: API conventions
on: pull_request
jobs:
  grep:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          set -e
          ! grep -rnE 'gin\.H\{"(error|success|data|message)":' \
            internal/ cmd/ \
            --include='*.go' \
            --exclude='*_test.go'
          ! grep -rnE '\.(GET|POST|PUT|DELETE|PATCH)\([^)]*"[^"]*:[a-z]+_[a-z_]+' \
            internal/ cmd/ \
            --include='*.go'
```

The exclamation mark inverts the exit code so the step fails when grep
finds a hit.

---

## 10. Workspace-level CI conformance gate

Lives at `infra-platform/.github/workflows/api-conformance.yml`. It:

1. Clones every service repo + `bruno-tests` into a workspace layout.
2. Boots the full Niaga stack via `docker compose up -d --wait`.
3. Seeds the admin user from `postgres/03-create-admin.sql`.
4. Runs the Bruno smoke collection per service via
   `npx -y @usebruno/cli@latest run`.

Each `.bru` file already asserts `res.body.success: eq true` and
shape-specific properties. Bruno CLI exits non-zero on any failure and
the workflow fails accordingly. Add new probes by dropping `.bru` files
into `bruno-tests/<service>/smoke/` (use `/bruno-add` to scaffold).

Triggers:

- `pull_request` on infra-platform itself (whenever
  `nginx/`, `docker-compose.yml`, or postgres seed SQL changes).
- `schedule` daily — drift detection across all service mains.
- `workflow_dispatch` manual runs and `workflow_call` reuse from
  service-repo PR workflows.

---

## 11. NATS event conventions

NATS events use the transactional outbox plus JetStream path:

- Publishers write `outbox.events` in the same database transaction as the
  business mutation. Do not publish directly from request handlers.
- `lib-common/outbox.Processor` publishes each outbox row to JetStream and
  sets `Nats-Msg-Id` to the outbox row UUID for retry deduplication.
- Use `nats.NewOutboxPublisher(jetStreamClient)` when wiring the processor to
  the shared JetStream client.
- Subjects use `events.<domain>.<action>` with a stable dotted action segment
  when needed, for example `events.order.created`,
  `events.catalog.product.updated`, and `events.support.ticket.resolved`.
- Domain events include `schema_version`; additive payload changes bump this
  field, while breaking changes require a `.v2` subject and a dual-publish
  window.
- Consumers use durable JetStream consumers with explicit ack, then call
  `eventsourcing.IdempotencyChecker.CheckAndMark(ctx, eventID, consumerName)`
  before running the handler. Duplicates are acked and skipped.
- Failed messages are routed through `eventsourcing.DLQRouter`, which inserts
  into `events.failed` and terminates JetStream redelivery.
- Durable consumer names are permanent. Do not rename a durable consumer after
  it has shipped; create a new consumer only when a replay boundary is intended.
- Ordering is only promised per aggregate and per subject. Global ordering is
  not part of the contract.
- New consumers use `DeliverNewPolicy` by default. Manual replay comes from
  `events.failed` rows or an explicit JetStream replay tool.

Canonical subjects live in `eventsourcing/catalog.go`.

---

## 12. What this doc does NOT cover

- **GraphQL / gRPC** — Niaga is REST-only.
- **API gateway swap** — nginx stays; no Traefik/Kong.
- **Multi-tenant schema changes** — out of scope for v1
  standardization.
- **TS client codegen from OpenAPI** — deferred until lib-ui
  consolidation lands.

---

## Related

- Plan: `.claude/memory/api-standardization-plan.md` (workspace).
- Response helpers: [`response/README.md`](response/README.md).
- Bruno smoke: `bruno-tests/` (collection); see
  `.claude/memory/bruno-plan.md` (workspace) for conventions.
- Per-service OpenAPI: `<service>/docs/openapi.yaml`.
- Webhook envelope exemption: `feedback-webhook-envelope-exception.md`
  (workspace memory).
