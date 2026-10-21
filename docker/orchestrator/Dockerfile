# Build context is the repository root: docker build -f docker/orchestrator/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY services/orchestrator ./services/orchestrator
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/orchestrator ./services/orchestrator/cmd/orchestrator

FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=build /out/orchestrator /orchestrator
ENTRYPOINT ["/orchestrator"]
CMD ["serve"]
