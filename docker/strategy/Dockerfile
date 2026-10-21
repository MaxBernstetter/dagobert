# Build context is the repository root: docker build -f docker/strategy/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY strategy ./strategy
COPY internal/instruments ./internal/instruments
COPY strategy/schema/playbook.schema.json strategy/internal/playbook/playbook.schema.json
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/strategy ./strategy/cmd/strategy

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/strategy /strategy
ENTRYPOINT ["/strategy"]
