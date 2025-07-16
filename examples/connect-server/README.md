# Connect Server Example

This example demonstrates how to create a Connect-aware gRPC server using the dgrpc library with panic recovery interceptors.

## Features

- **Connect Protocol Support**: Serves gRPC services over Connect protocol (HTTP/JSON)
- **Panic Recovery**: Includes Connect-compatible panic recovery interceptor
- **Health Checks**: HTTP health endpoint at `/healthz`
- **CORS Support**: Permissive CORS configuration for web clients
- **Reflection**: gRPC reflection enabled for service discovery

## Running the Server

```bash
go run .
```

The server will start on `localhost:8080` and serve the PingPongService over Connect protocol.

## Testing the Server

### Health Check
```bash
curl http://localhost:8080/healthz
```

### Connect/gRPC-Web Request
```bash
curl -H 'Content-Type: application/json' \
     -d '{"clientId":"test","message":"hello"}' \
     http://localhost:8080/acme.v1.PingPongService/GetPing
```

### Trigger Panic Recovery
The GetPing method intentionally panics to demonstrate the panic recovery interceptor. The server will log the panic but continue serving other requests.

## Key Components

1. **Connect Service Implementation**: Custom implementation of the PingPongService for Connect protocol
2. **Panic Recovery Interceptor**: Connect-compatible interceptor that isolates panics to individual requests
3. **ConnectRPC Server**: Uses `dgrpc/server/connectrpc` for native Connect protocol support

## Notes

This example shows how to:
- Create Connect-compatible service handlers
- Apply panic recovery interceptors as pre-interceptors
- Configure CORS and reflection for web clients
- Handle both unary and streaming RPC methods over Connect protocol