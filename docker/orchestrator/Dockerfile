# syntax=docker/dockerfile:1
# Build context is the repository root: docker build -f docker/orchestrator/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY src/shared ./src/shared
COPY src/ingest/provider ./src/ingest/provider
COPY src/internal/awsenv ./src/internal/awsenv
COPY src/internal/lakeobject ./src/internal/lakeobject
COPY src/internal/candlefile ./src/internal/candlefile
COPY src/internal/parquetmeta ./src/internal/parquetmeta
COPY src/internal/secrets ./src/internal/secrets
COPY src/orchestrator ./src/orchestrator
COPY src/experimentapi ./src/experimentapi
COPY src/calculation-performance ./src/calculation-performance
COPY src/gen ./src/gen
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/orchestrator ./src/orchestrator/cmd/orchestrator

FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=build /out/orchestrator /orchestrator
ENTRYPOINT ["/orchestrator"]
CMD ["serve"]
