#pragma once

#include "position.hpp"
#include "sma.hpp"
#include "sma_cross_signal.hpp"
#include "strategy.hpp"

#include "tradingcopilot/v1/run_envelope.pb.h"

namespace dagobert {

class SmaCrossStrategy : public Strategy {
public:
  explicit SmaCrossStrategy(const tradingcopilot::v1::RunEnvelope &envelope,
                            std::int64_t bar_duration_ms);

  [[nodiscard]] StrategyResult
  calc_backtest(std::span<const Bar> bars) override;
  [[nodiscard]] StrategyResult calc_live(const Bar &bar) override;
  void reset() override;

private:
  SMA fast_;
  SMA slow_;
  SmaCrossSignal cross_;
  Position book_;
};

} // namespace dagobert
