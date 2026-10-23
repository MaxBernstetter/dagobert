# syntax=docker/dockerfile:1
# Build context is the repository root: docker build -f docker/calculation-signal/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY src/shared ./src/shared
COPY src/calculation-signal ./src/calculation-signal
COPY src/internal ./src/internal
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/calculation-signal ./src/calculation-signal/cmd/calculation-signal

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/calculation-signal /calculation-signal
ENTRYPOINT ["/calculation-signal"]
