# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS builder

WORKDIR /src

# Cache deps separately
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ai-shell .

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates bash git curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/ai-shell /usr/local/bin/ai-shell

WORKDIR /workspace

# Persist config/data inside image volumes by default; override with bind mounts in compose
ENV HOME=/root

ENTRYPOINT ["ai-shell"]
