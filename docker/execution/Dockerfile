# Build context is the repository root: docker build -f docker/execution/Dockerfile .
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY services/execution ./services/execution
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/execution ./services/execution/cmd/execution

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/execution /execution
ENTRYPOINT ["/execution"]
