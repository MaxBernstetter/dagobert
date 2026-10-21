# Build context is the repository root: docker build -f docker/strategy/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY src/strategy ./src/strategy
COPY src/internal/instruments ./src/internal/instruments
COPY src/strategy/schema/playbook.schema.json src/strategy/internal/playbook/playbook.schema.json
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/strategy ./src/strategy/cmd/strategy

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/strategy /strategy
ENTRYPOINT ["/strategy"]
