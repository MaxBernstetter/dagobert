# syntax=docker/dockerfile:1
# Build context is the repository root: docker build -f docker/ingest/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY src/shared ./src/shared
COPY src/internal/awsenv ./src/internal/awsenv
COPY src/internal/lakeobject ./src/internal/lakeobject
COPY src/internal/ingestledger ./src/internal/ingestledger
COPY src/internal/pgpool ./src/internal/pgpool
COPY src/internal/candlefile ./src/internal/candlefile
COPY src/internal/parquetmeta ./src/internal/parquetmeta
COPY src/ingest ./src/ingest
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/ingest ./src/ingest/cmd/ingest

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/ingest /ingest
ENTRYPOINT ["/ingest"]
