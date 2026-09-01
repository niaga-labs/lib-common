# Changelog

All notable changes to `lib-common` are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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
