# syntax=docker/dockerfile:1.7

# -----------------------------------------------------------------------------
# Stage 1: Build static Go 1.26 binary (CGO_ENABLED=0)
# -----------------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /out/sdui-bff \
    ./cmd/sdui-bff

# -----------------------------------------------------------------------------
# Stage 2: Minimal distroless non-root runtime image
# -----------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="abeta-sdui-bff" \
      org.opencontainers.image.description="Abeta Multi-Vertical SDUI Domain BFF powering 6 stateful vertical apps for Abeta-dev/kmp" \
      org.opencontainers.image.source="https://github.com/Abeta-dev/sdui-bff" \
      org.opencontainers.image.licenses="Apache-2.0" \
      dev.abeta.healthcheck.liveness="/healthz" \
      dev.abeta.healthcheck.readiness="/readyz" \
      dev.abeta.healthcheck.port="8085" \
      dev.abeta.healthcheck.scheme="http"

# Note: gcr.io/distroless/static-debian12:nonroot ships without a shell or wget/curl
# to keep the attack surface minimal. Orchestrators (Kubernetes, ECS, Nomad, Compose)
# should probe HTTP GET http://127.0.0.1:8085/healthz (liveness) and /readyz (readiness).

WORKDIR /app
COPY --from=builder /out/sdui-bff /app/sdui-bff

ENV PORT=8085 \
    GIN_MODE=release \
    LOG_LEVEL=info \
    RATE_LIMIT_RPS=0

EXPOSE 8085

USER nonroot:nonroot

ENTRYPOINT ["/app/sdui-bff"]
