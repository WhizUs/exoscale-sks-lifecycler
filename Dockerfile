# syntax=docker/dockerfile:1

FROM golang:1.21.9-alpine3.19 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go cmd ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /exoscale-sks-lifecycler

FROM alpine:3.19.1 AS production

LABEL org.opencontainers.image.source="https://github.com/WhizUs/exoscale-sks-lifecycler" \
    org.opencontainers.image.url="https://github.com/WhizUs/exoscale-sks-lifecycler" \
    org.opencontainers.image.title="exoscale-sks-lifecycler" \
    org.opencontainers.image.vendor='The exoscale-sks-lifecycler Authors' \
    org.opencontainers.image.licenses='Apache-2.0'

RUN apk add --no-cache ca-certificates

WORKDIR /
COPY --from=builder /exoscale-sks-lifecycler .
USER 65532:65532

CMD ["/exoscale-sks-lifecycler", "nodepool", "cycle"]
