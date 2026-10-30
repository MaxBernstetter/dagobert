#pragma once

#include "atr.hpp"
#include "position.hpp"
#include "sma.hpp"
#include "sma_cross_signal.hpp"
#include "strategy.hpp"

#include "proto_fwd.hpp"

namespace dagobert {

class SmaCrossStrategy : public Strategy {
public:
  // With `sizing` the strategy runs the position-sizing flow (stop, cut,
  // pyramid) in backtest and live alike; without it, buy_all/sell_all.
  explicit SmaCrossStrategy(
      const tradingcopilot::v1::RunEnvelope &envelope,
      std::int64_t bar_duration_ms,
      std::optional<sizing::SizingConfig> sizing = std::nullopt);

  // The envelope's optional `sizing` block; exits 2 when it is invalid.
  [[nodiscard]] static std::optional<sizing::SizingConfig>
  sizing_of(const tradingcopilot::v1::RunEnvelope &envelope);

  [[nodiscard]] StrategyResult
  calc_backtest(std::span<const Bar> bars) override;
  [[nodiscard]] StrategyResult calc_live(const Bar &bar) override;
  void reset() override;

private:
  SMA fast_;
  SMA slow_;
  ATR atr_;
  std::optional<sizing::SizingConfig> sizing_;
  SmaCrossSignal cross_;
  Position book_;
};

} // namespace dagobert
