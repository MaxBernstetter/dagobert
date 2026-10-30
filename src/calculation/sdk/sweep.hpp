#pragma once

#include "strategy.hpp"

#include "proto_fwd.hpp"

#include <functional>
#include <iosfwd>
#include <memory>

namespace dagobert {

struct SweepStrategy {
  std::unique_ptr<Strategy> strategy;
  bool sized{false};
};

// Builds one item's strategy from its envelope (base + item payload). A bad
// config throws ConfigError.
using SweepFactory = std::function<SweepStrategy(
    const tradingcopilot::v1::RunEnvelope &, std::int64_t bar_duration_ms)>;

// `--sweep`: reads one SweepRequest from `in`, runs every item over the shared
// bars and writes one length-delimited RunReport per item to `out`. A bad item
// yields a report with `error` set. Returns the process exit code: 2 for an
// unreadable request, 1 for no bars or a write failure, else 0.
[[nodiscard]] int run_sweep(std::istream &in, std::ostream &out,
                            const SweepFactory &factory);

} // namespace dagobert
