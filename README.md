# StreamingFast  GRPC Library

[![reference](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://pkg.go.dev/github.com/streamingfast/dgrpc)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This is a helper library for instanciating gRPC clients and servers. It is part of **[StreamingFast](https://github.com/streamingfast/streamingfast)**.

## Usage

See examples usage in [examples folder](./examples).

## Logging

If you define the environment variable `GRPC_REGISTER_ZAP_LOGGER`, the `dgrpc` when imported will immediately replace the standard `golang.google.com/grpc/grpclog` package so that `grpc-go` logging goes through StreamingFast standard `github.com/streamingfast/logging` library and you can then dynamically configure the logger level using `DLOG` spec.

The default registered logger has short name `grpc` and its id is `google.golang.org/grpc`.

If `GRPC_REGISTER_ZAP_LOGGER` was set on startup (any value is accepted), you should see rapidly gRPC internal logs printed to your console.

> [!NOTE]
> We do not register the zap logger into gRPC stack on program initialization because it prints a lot of warning which would most probably clutter the logs by default. It make sense to set `GRPC_REGISTER_ZAP_LOGGER` by default if you provide a spec that ignores logs from `google.golang.org/grpc` by default.

Once `GRPC_REGISTER_ZAP_LOGGER` is set, use standard `logging` spec like `DLOG="google.golang.org/grpc=debug"` and you will see logs appearing. At any moment you can call `dgrpc.SetGRPCLogger` (or `dgrpc.SetGRPCLoggerWithVerbosity`) to override the active `grpc-go` `*zap.Logger` with your own.

## Notes

As of commit `49c1ad3ecbaa5ab09850bdd555cc6d8422b6911b`, the http handler for `connect-web/server` has changed from `net/http` to `github.com/gorilla/mux`. Please refer to https://github.com/gorilla/mux when creating a handler.

Here is an example on how to do it:
```bash
options := []dgrpcserver.Option{
    [...]
}
options = append(options,
    dgrpcserver.WithConnectWebHTTPHandlers([]dgrpcserver.HTTPHandlerGetter{
        func() (string, http.Handler) {
            return "/auth/callback", authCallback
        },

        func() (string, http.Handler) {
            return "/articles/{id}/{articleName}", articlesCallback
        },
    }),
)
[...]

srv := connectweb.New([]connectweb.HandlerGetter{...httpHandlers}, options...)
[...]
srv.Launch(addr)
```

## Contributing

**Issues and PR in this repo related strictly to the dgrpc library.**

Report any protocol-specific issues in their
[respective repositories](https://github.com/streamingfast/streamingfast#protocols)

**Please first refer to the general
[StreamingFast contribution guide](https://github.com/streamingfast/streamingfast/blob/master/CONTRIBUTING.md)**,
if you wish to contribute to this code base.

This codebase uses unit tests extensively, please write and run tests.


## License

[Apache 2.0](LICENSE)
