#pragma once

#include "quote.hpp"
#include "types.hpp"

#include "proto_fwd.hpp"

#include <cstdint>
#include <span>

namespace dagobert {

// Current RunEnvelope/RunReport contract version.
inline constexpr std::uint32_t kSchemaVersion = 2;

class Strategy {
public:
  explicit Strategy(const tradingcopilot::v1::RunEnvelope &envelope);
  virtual ~Strategy() = default;

  [[nodiscard]] virtual StrategyResult
  calc_backtest(std::span<const Bar> bars) = 0;
  [[nodiscard]] virtual StrategyResult calc_live(const Bar &bar) = 0;
  virtual void reset() = 0;

  [[nodiscard]] const Quote &initial_quote() const { return initial_quote_; }

  // Bars opening before this are warm-up only: calc_backtest takes no action
  // on them (sweep mode). Default: trade from the first bar.
  void set_trade_from(std::int64_t open_ms) { trade_from_ms_ = open_ms; }
  [[nodiscard]] std::int64_t trade_from_ms() const { return trade_from_ms_; }

protected:
  static void
  require_schema_version(const tradingcopilot::v1::RunEnvelope &envelope);

private:
  Quote initial_quote_;
  std::int64_t trade_from_ms_{INT64_MIN};
};

} // namespace dagobert
