# syntax=docker/dockerfile:1
# Build context is the repository root: docker build -f docker/experiment-api/Dockerfile .
FROM golang:1.24-alpine AS build
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY src ./src
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/bootstrap ./src/cmd/experiment-api-lambda

FROM public.ecr.aws/lambda/provided:al2023
COPY --from=build /out/bootstrap /var/runtime/bootstrap
ENTRYPOINT ["/var/runtime/bootstrap"]
