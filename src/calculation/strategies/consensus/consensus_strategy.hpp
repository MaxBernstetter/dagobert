#pragma once

#include "atr.hpp"
#include "consensus_signal.hpp"
#include "macd.hpp"
#include "position.hpp"
#include "rsi.hpp"
#include "sma.hpp"
#include "strategy.hpp"

#include "tradingcopilot/v1/run_envelope.pb.h"

namespace dagobert {

class ConsensusStrategy : public Strategy {
public:
  // With `sizing` the entry, stop, vol_cut and pyramid flow runs in backtest
  // and live alike; without it, buy_all/sell_all.
  explicit ConsensusStrategy(
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
  SMA signal_;
  SMA regime_;
  SMA macro_;
  RSI rsi_;
  SMA vol_mean_;
  ATR atr_;
  MACD macd_;
  std::optional<sizing::SizingConfig> sizing_;
  ConsensusSignal consensus_;
  Position book_;
};

} // namespace dagobert
