# StreamingFast  GRPC Library

[![reference](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://pkg.go.dev/github.com/streamingfast/dgrpc)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

This is a helper library for instanciating GRPC clients and servers.
It is part of **[StreamingFast](https://github.com/streamingfast/streamingfast)**.


## Usage

See example usage in [bstream](https://github.com/streamingfast/bstream).

## Contributing

**Issues and PR in this repo related strictly to the dgrpc library.**

Report any protocol-specific issues in their
[respective repositories](https://github.com/streamingfast/streamingfast#protocols)

**Please first refer to the general
[StreamingFast contribution guide](https://github.com/streamingfast/streamingfast/blob/master/CONTRIBUTING.md)**,
if you wish to contribute to this code base.

This codebase uses unit tests extensively, please write and run tests.

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

## License

[Apache 2.0](LICENSE)
