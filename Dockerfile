# syntax=docker/dockerfile:1.6

# Build stage
FROM golang:1.22-alpine AS build
WORKDIR /src

# Optional: install git for fetching modules from VCS
RUN apk add --no-cache git

# Cache deps first
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source
COPY . .

# Enable cross-platform builds via Buildx-provided args
ARG TARGETOS
ARG TARGETARCH

# Build static binary for target platform
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/app .

# Runtime stage (distroless includes CA certs)
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]