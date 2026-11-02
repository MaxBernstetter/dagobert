# syntax=docker/dockerfile:1

# Stage 1: shared C++ dependency prefix (Arrow, Parquet, protobuf, ...). Only
# the recipe files are copied in, so this layer is rebuilt only when the recipe
# changes, never for source edits. Portable code (no -march=native).
FROM debian:trixie AS cpp-deps
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential clang lld cmake ninja-build git ca-certificates \
      util-linux pkg-config python3 && rm -rf /var/lib/apt/lists/*
WORKDIR /calc
COPY src/calculation/deps ./deps
COPY src/calculation/cmake/DepsKey.cmake src/calculation/cmake/ApplyArrowPatches.cmake \
     src/calculation/cmake/Ccache.cmake src/calculation/cmake/ccache-launcher.sh ./cmake/
ENV DAGOBERT_DEPS_ROOT=/deps DAGOBERT_PORTABLE=1
# Arrow's bundled snappy does not compile with Debian's clang 19 (__m256i), so
# the dependency prefix is built with gcc; libstdc++ is shared with the clang build below.
ENV DAGOBERT_CC=gcc DAGOBERT_CXX=g++
RUN deps/build.sh optimized

# Stage 2: the C++ strategy executables against that prefix.
FROM debian:trixie AS cpp-build
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential clang lld cmake ninja-build git ca-certificates \
      util-linux pkg-config python3 && rm -rf /var/lib/apt/lists/*
COPY --from=cpp-deps /deps /deps
COPY src/calculation /calc
ENV DAGOBERT_DEPS_ROOT=/deps
RUN cmake -S /calc -B /out -G Ninja -DDAGOBERT_MODE=optimized -DDAGOBERT_PORTABLE=ON -DDAGOBERT_LINKER=lld \
      -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_COMPILER=clang -DCMAKE_CXX_COMPILER=clang++ && \
    cmake --build /out --target strategy-sma-cross strategy-signal-consensus

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
    go build -trimpath -ldflags="-s -w" -o /out/backtest ./src/cmd/backtest

FROM gcr.io/distroless/cc-debian13
COPY --from=build /out/backtest /backtest
COPY --from=cpp-build /out/strategies/sma-cross/strategy-sma-cross /usr/local/bin/strategy-sma-cross
COPY --from=cpp-build /out/strategies/consensus/strategy-signal-consensus /usr/local/bin/strategy-signal-consensus
ENV CPP_SMA_CROSS_BIN=/usr/local/bin/strategy-sma-cross \
    CPP_SIGNAL_CONSENSUS_BIN=/usr/local/bin/strategy-signal-consensus
ENTRYPOINT ["/backtest"]
