# Build context is the repository root: docker build -f docker/process/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY src/process ./src/process
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/process ./src/process/cmd/process

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/process /process
ENTRYPOINT ["/process"]
