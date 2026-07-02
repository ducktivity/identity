# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Download dependencies first so this layer caches across source-only changes. All modules (including github.com/ducktivity/platform-go) are public, so this fetches over the default proxy with no token or git credential config.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build metadata, supplied by CI from git and injected via -ldflags so logs and the Sentry release report the exact running commit.
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# Static binary so it runs on the distroless/static base.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o /out/identity ./

# ---- Runtime stage ----
# distroless/static ships CA certificates (outbound TLS to Neon, Resend, and Sentry) and runs as non-root, with no shell or package manager.
FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/identity /app/identity
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/app/identity"]
