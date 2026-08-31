# Changelog

All notable changes to `lib-common` are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Notes

- Shared Go library consumed by every service as a module dependency; it has no `cmd/` and no port.
- 6 `*_test.go` files — the only repo besides `service-marketplace` with any tests at all.
- Owns `events` and `outbox` in `niaga_db` (the transactional outbox the services publish through).
