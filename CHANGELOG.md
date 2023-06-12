# Change log

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

* Removed `WithAuthChecker` support. Use `PostUnaryInterceptors` & `PostStreamInterceptors` to add an authentication interceptor
* Added `WithRegisterService` on `dgrpc.Server` (which is obtained via `dgrpc.NewServer2`).
* Introduced a `NewServer2` that embeds more power inside a thin-wrapper `dgrpc.Server` struct that wraps both a HTTP server and a gRPC server. The HTTP server is started only when `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)` option is used. The `NewServer2` handles more configurability option like TLS config, health check, and many more.
* **Deprecation** The `dgrpc.NewServer` is deprecated, it will be replaced by another implementation in the future, replaces with a call to `dgrpc.NewGRPCServer` instead.
* **Deprecation** The `dgrpc.SimpleHealthCheck` is deprecated uses `server := dgrpc.NewServer2(options...)` with the `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)` option then `go server.Launch()`
* **Deprecation** The `dgrpc.SimpleHTTPServer` is deprecated sses `server := dgrpc.NewServer2(options...)` with the `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)` then `go server.Launch()` instead.
* **Deprecation** The `dgrpc.ListenAndServe` is deprecated sses `server := dgrpc.NewServer2(options...)` with the `dgrpc.WithHealthCheck(dgrpc.HealthCheckOverHTTP, ...)` then `go server.Launch()` instead.

## 2020-03-21

### Changed

* License changed to Apache 2.0