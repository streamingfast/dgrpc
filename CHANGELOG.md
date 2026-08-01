# Change log

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

- Add zstd support for connectrpc server
- Remove connectrpc error middleware by default. You need to add it `connectrpc.NewErrorsInterceptor`
- Added abiliy to map connectrpc error
- Removed `WithAuthChecker` support. Use `PostUnaryInterceptors` & `PostStreamInterceptors` to add an authentication interceptor
- Added `WithRegisterService` on `dgrpc.Server` (which is obtained via `dgrpc.NewServer2`).
- Introduced a `NewServer2` that embeds more power inside a thin-wrapper `dgrpc.Server` struct that wraps both a HTTP server and a gRPC server. The HTTP server is started only when `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)` option is used. The `NewServer2` handles more configurability option like TLS config, health check, and many more.
- **Deprecation** The `dgrpc.NewServer` is deprecated, it will be replaced by another implementation in the future, replaces with a call to `dgrpc.NewGRPCServer` instead.
- **Deprecation** The `dgrpc.SimpleHealthCheck` is deprecated uses `server := dgrpc.NewServer2(options...)` with the `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)` option then `go server.Launch()`
- **Deprecation** The `dgrpc.SimpleHTTPServer` is deprecated sses `server := dgrpc.NewServer2(options...)` with the `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)` then `go server.Launch()` instead.
- **Deprecation** The `dgrpc.ListenAndServe` is deprecated sses `server := dgrpc.NewServer2(options...)` with the `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)` then `go server.Launch()` instead.
- move from deprecated `github.com/bufbuild/connect-go` to `connectrpc.com/connect`

### Added

- Add `List` to `server.HealthGRPCHandler` and `server/connectrpc.HealthGRPCHandler`, implementing the `grpc.health.v1.Health/List` RPC added to the `HealthServer` interface in gRPC-Go v1.82.0. Both return a single entry keyed by the empty service name, matching the server-wide health check these handlers expose.

### Changed

- Bump `google.golang.org/grpc` from v1.77.0 to v1.83.0.
- **Breaking** Raise the minimum Go version to 1.25.0, as required by gRPC-Go v1.82.0 and above.
- Demote the per-request `compression enabled` log from Info to Debug — it fired on every request and carried no actionable signal.

### Security

- Bump `golang.org/x/crypto` from v0.51.0 to v0.54.0, clearing the `golang.org/x/crypto/ssh` advisories (GO-2026-5033, GO-2026-5023, GO-2026-5021, GO-2026-5020, GO-2026-5019, GO-2026-5018) that were pulled in transitively by the gRPC-Go v1.83.0 bump. These are fixed in v0.52.0; `dgrpc` does not import `crypto/ssh`, so no code path was reachable, but the module requirement tripped dependency scanners.
- Bump companion `golang.org/x` modules alongside it: `net` v0.55.0 → v0.56.0, `sync` v0.20.0 → v0.22.0, `sys` v0.45.0 → v0.47.0, `term` v0.43.0 → v0.45.0, `text` v0.37.0 → v0.40.0.

## 2020-03-21

### Changed

- License changed to Apache 2.0
