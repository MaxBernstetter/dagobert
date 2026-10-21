# Build context is the repository root: docker build -f docker/execution/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY src/execution ./src/execution
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/execution ./src/execution/cmd/execution

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/execution /execution
ENTRYPOINT ["/execution"]
