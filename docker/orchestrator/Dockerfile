# syntax=docker/dockerfile:1
# Build context is the repository root: docker build -f docker/orchestrator/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY src ./src
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/orchestrator ./src/orchestrator/cmd/orchestrator

FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=build /out/orchestrator /orchestrator
ENTRYPOINT ["/orchestrator"]
CMD ["serve"]
