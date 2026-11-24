# syntax=docker/dockerfile:1

# Build from the repository root so the examples compile against the local dgrpc sources (go.work):
#
#   docker build -f examples/Dockerfile -t dgrpc-examples .
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add build-base

COPY . .

ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/plain-text-server ./examples/plain-text-server && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/secure-server ./examples/secure-server && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/connect-server ./examples/connect-server && \
    echo "Done"

FROM alpine

RUN apk --no-cache add openssl ca-certificates curl

RUN mkdir /app

COPY --from=builder /app/plain-text-server /app/
COPY --from=builder /app/secure-server /app/
COPY --from=builder /app/connect-server /app/

WORKDIR /app

EXPOSE 9000 8080
