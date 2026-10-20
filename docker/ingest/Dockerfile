# Build context is the repository root: docker build -f docker/ingest/Dockerfile .
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY services/ingest ./services/ingest
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/ingest ./services/ingest/cmd/ingest

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/ingest /ingest
ENTRYPOINT ["/ingest"]
