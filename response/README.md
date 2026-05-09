# `lib-common/response` — Niaga API response envelope

Canonical HTTP response envelope used by every Niaga service. **Every public API response goes through one of the helpers in `response.go`. Never `c.JSON(gin.H{...})` directly.**

This package + its conventions are the source of truth for the [API standardization plan](../../.claude/memory/api-standardization-plan.md). When in doubt, this README wins over any service-local pattern.

## Envelope shape

```jsonc
{
  "success": true,
  "message": "Optional human-readable summary",
  "data": { /* domain payload, snake_case keys */ },
  "error": null,                           // present only on failure
  "meta": {                                // present only on paginated responses
    "page": 1,
    "limit": 20,
    "total_pages": 5,
    "total_count": 87,
    "total": 87                            // alias for total_count, frontend convenience
  }
}
```

Fail case:

```jsonc
{
  "success": false,
  "error": {
    "code": "ORDER_NOT_FOUND",
    "message": "The requested order was not found.",
    "details": null                        // structured details if any (e.g. validation field map)
  }
}
```

## JSON casing — snake_case

**All JSON tags use `snake_case`.** That includes top-level envelope keys (`total_pages`, `total_count`) and nested payload keys (`created_at`, `customer_id`, `order_status`).

Why: 8 of 9 services already serialize this. snake_case is more grep-friendly, matches Postgres column names directly, and avoids the camelCase-vs-snake_case dance inside Go services.

**The frontend consumes camelCase** — that conversion happens at the BFF proxy layer (`frontend-admin/src/app/api/proxy/[...path]/route.ts` and `frontend-storefront/app/api/proxy/[...path]/route.ts`), not in the services. Each service stays snake_case end-to-end.

If you find a service shipping camelCase tags (e.g. `service-auth`'s legacy `User.firstName`), that's a bug — open a PR to flip the tag.

## Helpers

Use these. **Never** `c.JSON(http.StatusXxx, gin.H{...})` directly.

### Success

```go
response.OK(c, "Order fetched", order)                      // 200 + data
response.Created(c, "Order created", order)                 // 201 + data
response.NoContent(c)                                        // 204 (DELETE/PUT-no-return)
response.List(c, "Orders", orders)                          // 200 + list (no meta)
response.Paginated(c, orders, page, limit, totalCount)      // 200 + list + Meta
```

### Errors

```go
response.BadRequest(c, "Invalid query", validationDetails)   // 400
response.Unauthorized(c, "Missing token")                    // 401
response.Forbidden(c, "Admin only")                          // 403
response.NotFound(c, "Order not found")                      // 404
response.Conflict(c, "Email already in use")                 // 409
response.ValidationError(c, "Validation failed", fieldMap)   // 422
response.TooManyRequests(c, "Slow down", retryAfterDetails)  // 429
response.InternalServerError(c, "DB unreachable")            // 500
response.ServiceUnavailable(c, "Maintenance")                // 503
```

### Domain error translation

For services that translate domain errors to HTTP responses (e.g. service-order), use the `ErrorTranslator`:

```go
translator := response.DefaultOrderErrorTranslator()
// or build your own:
translator := response.NewErrorTranslator().
    Register("user not found", response.ErrorMapping{
        HTTPStatus: http.StatusNotFound,
        Code:       "USER_NOT_FOUND",
        Message:    "The requested user was not found.",
    })

translator.Translate(c, err, logger)                         // emits Error(...) with the mapping
```

This prevents leaking internal error strings to clients.

## Pagination contract

Every paginated list endpoint accepts `?page=1&limit=20` query params and returns the `Meta` block above.

**Don't use** `?page_size=`, `?offset=`, or any other pagination shape. The API standardization plan explicitly drops those.

The `Meta.total` field is a duplicate of `Meta.total_count` — it's an alias kept for FE convenience; new clients should prefer `total_count`.

## Error code conventions

- Codes are `SCREAMING_SNAKE_CASE` (e.g. `ORDER_NOT_FOUND`, `INSUFFICIENT_STOCK`)
- Codes are stable contracts — once shipped, treat them like API surface. Renames are breaking changes.
- Prefer specific codes (`PAYMENT_FAILED`) over generic (`BAD_REQUEST`) when the failure mode is well-defined
- `details` field carries structured info (field-level validation errors, retry hints, etc.) — never raw stack traces or DB errors

## What this package does NOT do

- **Authentication** — see `lib-common/middleware/auth.go`
- **Request-ID propagation** — see `lib-common/middleware/requestid.go`
- **CORS / validation / rate limiting** — see `lib-common/middleware/`
- **Tracing / Sentry** — see `lib-common/telemetry/` and `lib-common/monitoring/`
- **Casing transforms** — that's the BFF proxy's job (FE-facing layer only)

## Related

- Source of truth for the conventions that drive this package: `.claude/memory/api-standardization-plan.md` in the workspace
- BFF proxy that converts to camelCase for the frontend: `frontend-admin/src/app/api/proxy/[...path]/route.ts` and `frontend-storefront/app/api/proxy/[...path]/route.ts`
- Bruno smoke probes that pin this envelope shape: `bruno-tests/` (see `bruno-plan.md`)
