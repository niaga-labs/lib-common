# Changelog

All notable changes to `lib-common` are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `.github/workflows/go-ci.yml` — one reusable CI workflow for every Go repo in the org (gofmt · vet · build ·
  test). The Go version is read from the caller's `go.mod` so it cannot drift; each check can be relaxed per
  repo while existing debt is cleared. (DMB-19)

### Fixed

- **CI could not build any service, so Lint and Test failed in every Go repo — including pull requests that
  only added a CHANGELOG.** Each service reaches this library through `replace github.com/niaga-platform/lib-common
  => ../lib-common`, a path that exists only on a developer's laptop. A CI runner checks out one repository, so
  the module never resolved: `golangci-lint` exited 3 with *"could not load export data"* and `go test` failed
  to build. `main` had been red for many commits, so this was never PR damage. The workflow now checks
  `lib-common` out into `.lib-common` inside the workspace and repoints the replace at it with `go mod edit`,
  which touches only the runner's copy of `go.mod`. `gofmt` skips that directory; `go vet`, `build` and `test`
  already ignore it because it carries its own `go.mod`. (DMB-69)

### Notes

- A repo whose `go.mod` does not mention `lib-common` skips the extra checkout, so this library's own CI is
  unaffected.
- Reading a second **private** repository needs a token the default `GITHUB_TOKEN` cannot provide — it is
  scoped to the calling repository. The workflow accepts an optional `LIB_COMMON_TOKEN` secret and falls back
  to `github.token`. Until that secret exists at org level (or `lib-common` becomes public), the checkout step
  is the one thing that can still fail. DMB-69 carries the decision.

- Shared Go library consumed by every service as a module dependency; it has no `cmd/` and no port.
- 6 `*_test.go` files — the only repo besides `service-marketplace` with any tests at all.
- Owns `events` and `outbox` in `niaga_db` (the transactional outbox the services publish through).
