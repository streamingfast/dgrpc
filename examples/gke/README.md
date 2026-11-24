# GKE: server HTTP/2 PINGs behind a Google load balancer

Minimal setup to observe how a Google external Application Load Balancer interacts with the server
HTTP/2 PINGs (`server.WithHTTP2Pings`) of dgrpc servers: `client <=> Google LB <=> pod (TLS, self-signed)`.

## Deploy

```bash
# From the repository root
docker build -f examples/Dockerfile -t REGISTRY/dgrpc-examples:dev . && docker push REGISTRY/dgrpc-examples:dev

# Frontend certificate (self-signed is fine for testing, use -k / --insecure on clients)
openssl req -x509 -newkey rsa:2048 -nodes -days 30 -subj "/CN=dgrpc-pings" \
  -keyout /tmp/key.pem -out /tmp/cert.pem
kubectl create secret tls dgrpc-pings-tls --cert=/tmp/cert.pem --key=/tmp/key.pem

# Replace REGISTRY in deployment.yaml first
kubectl apply -f examples/gke/
kubectl get ingress dgrpc-pings -w   # wait for an ADDRESS, then a few minutes for the LB to become healthy
```

## Observe

`GODEBUG=http2debug=2` is set on the pod, follow the HTTP/2 frames with `kubectl logs -f deploy/dgrpc-pings`:

- The load balancer `SETTINGS` frame, e.g. `MAX_CONCURRENT_STREAMS` it advertises.
- Our `PING` after 30s idle (use `server.WithHTTP2Pings` with a small value to iterate faster) and the
  `PING flags=ACK` coming back. No ack would mean the connection is closed ~45s later.
- With a long stream open through the LB (`StreamPing` RPC, e.g. with `grpcurl -insecure`) for more than
  10 minutes, whether the connection survives the load balancer's fixed 600s backend idle timeout.

To switch the server under test, change `command` to `/app/connect-server` in `deployment.yaml`.

Both example servers read `LISTEN_ADDR`; a trailing `*` (e.g. `:9000*`) serves TLS with the built-in snake oil certificate.
