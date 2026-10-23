# syntax=docker/dockerfile:1
# Build context is the repository root.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY src/shared ./src/shared
COPY src/internal ./src/internal
COPY src/calculation-signal ./src/calculation-signal
COPY src/calculation-market-action ./src/calculation-market-action
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/calculation-market-action ./src/calculation-market-action/cmd/calculation-market-action

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/calculation-market-action /calculation-market-action
ENTRYPOINT ["/calculation-market-action"]
