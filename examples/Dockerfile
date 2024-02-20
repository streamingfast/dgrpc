# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./

ENV GOPRIVATE=github.com/streamingfast/hikari-bot

ARG TARGETOS TARGETARCH
# todo: move to its own step build
RUN apk add build-base
RUN --mount=target=. \
    --mount=type=ssh \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/plain-text-server ./plain-text-server && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/secure-server ./secure-server && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/xds-traffic-director ./xds-traffic-director && \
    echo "Done"

FROM alpine

RUN apk --no-cache add openssl ca-certificates curl

RUN mkdir /app

COPY --from=builder /app/plain-text-server /app/
COPY --from=builder /app/secure-server /app/
COPY --from=builder /app/xds-traffic-director /app/

WORKDIR /app

EXPOSE 9000
