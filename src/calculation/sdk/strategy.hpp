#pragma once

#include "quote.hpp"
#include "types.hpp"

#include "tradingcopilot/v1/run_envelope.pb.h"

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

protected:
  static void
  require_schema_version(const tradingcopilot::v1::RunEnvelope &envelope);

private:
  Quote initial_quote_;
};

} // namespace dagobert
