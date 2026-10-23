# syntax=docker/dockerfile:1
# Build context is the repository root: docker build -f docker/execution/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY src/shared ./src/shared
COPY src/internal ./src/internal
COPY src/execution ./src/execution
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/execution ./src/execution/cmd/execution

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/execution /execution
ENTRYPOINT ["/execution"]
