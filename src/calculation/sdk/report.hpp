#pragma once

#include "strategy.hpp" // IWYU pragma: keep

#include "proto_fwd.hpp" // IWYU pragma: keep

namespace dagobert {

// Fills `report` from a backtest result. `sized` adds the sizing fields to the
// actions and the ActionSizing list; `with_fills` is false in sweep mode.
void fill_report(const StrategyResult &result,
                 const tradingcopilot::v1::RunEnvelope &envelope,
                 std::int64_t duration_ms, bool sized, bool with_fills,
                 tradingcopilot::v1::RunReport &report);

} // namespace dagobert
