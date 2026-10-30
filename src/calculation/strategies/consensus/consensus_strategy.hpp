#pragma once

#include "consensus_features.hpp"
#include "consensus_signal.hpp"
#include "position.hpp"
#include "strategy.hpp"

#include "proto_fwd.hpp"

namespace dagobert {

class ConsensusStrategy : public Strategy {
public:
  // Bars are the Go playbook's decision bars: 3h aggregates of the lake
  // candles with their sub-bar closes (aggregate_bars). The indicator
  // definitions are the technical-indicator job's (ConsensusFeatures).
  // With `sizing` the entry, stop, vol_cut and pyramid flow runs in backtest
  // and live alike; without it, buy_all/sell_all.
  explicit ConsensusStrategy(
      const tradingcopilot::v1::RunEnvelope &envelope,
      std::int64_t bar_duration_ms,
      std::optional<sizing::SizingConfig> sizing = std::nullopt,
      const ConsensusParams &params = {});

  // Thresholds from the envelope's SignalConsensusConfig. An unset double (zero)
  // takes the Go playbook default; the cooldowns use field presence (0 is valid).
  // An invalid set exits 2.
  [[nodiscard]] static ConsensusParams
  params_of(const tradingcopilot::v1::RunEnvelope &envelope);

  // The envelope's optional `sizing` block; exits 2 when it is invalid.
  [[nodiscard]] static std::optional<sizing::SizingConfig>
  sizing_of(const tradingcopilot::v1::RunEnvelope &envelope);

  [[nodiscard]] StrategyResult
  calc_backtest(std::span<const Bar> bars) override;
  [[nodiscard]] StrategyResult calc_live(const Bar &bar) override;
  void reset() override;

private:
  ConsensusFeatures features_;
  ConsensusParams params_;
  std::optional<sizing::SizingConfig> sizing_;
  ConsensusSignal consensus_;
  Position book_;
};

} // namespace dagobert
