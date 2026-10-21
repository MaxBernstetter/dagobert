# Build context is the repository root: docker build -f docker/ingest/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY ingest ./ingest
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/ingest ./ingest/cmd/ingest

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/ingest /ingest
ENTRYPOINT ["/ingest"]
