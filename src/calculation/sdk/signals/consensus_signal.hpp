#pragma once

#include "consensus_gates.hpp"
#include "types.hpp"

#include <cstddef>
#include <optional>
#include <span>
#include <vector>

namespace dagobert {

class Position;

enum class ConsensusRule { consensus_buy, consensus_sell, atr_stop };

inline constexpr std::size_t kMinBarsFlatAfterSell = 8;
inline constexpr std::size_t kMinBarsLongBeforeSell = 4;

// Indicator columns the consensus signal reads (signal = SMA12, regime = SMA24,
// macro = SMA48, vol_mean = volume mean, hist = MACD histogram).
struct ConsensusColumns {
  const CloseSeries &close;
  const VolumeSeries &volume;
  const Column &signal;
  const Column &regime;
  const Column &macro;
  const Column &rsi;
  const Column &vol_mean;
  const Column &atr;
  const Column &hist;
};

// One bar of the same inputs for streaming use.
struct ConsensusSample {
  double close{};
  double volume{};
  std::optional<double> signal;
  std::optional<double> regime;
  std::optional<double> macro;
  std::optional<double> rsi;
  std::optional<double> vol_mean;
  std::optional<double> atr;
  std::optional<double> hist;
};

// Per-bar gate outcome before position state and cooldowns are applied.
struct ConsensusGate {
  bool entry{};
  bool cross_down{};
  bool atr_stop{};
};

struct ConsensusSeries {
  std::vector<ConsensusGate> gate;
};

class ConsensusSignal {
public:
  [[nodiscard]] static ConsensusSeries evaluate(const ConsensusColumns &in);

  [[nodiscard]] static std::vector<Action>
  actions_from(Position &account, std::span<const Bar> bars,
               const ConsensusSeries &mask);

  // Advances one bar; the caller executes the returned rule on its position.
  [[nodiscard]] std::optional<ConsensusRule> apply(const ConsensusSample &in,
                                                   bool flat);

  void execute(Position &account, const Bar &bar, ConsensusRule rule);
  void reset();

private:
  [[nodiscard]] std::optional<ConsensusRule> decide(const ConsensusGate &gate,
                                                    bool flat);

  BaselineCross cross_;
  RsiVote rsi_vote_;
  std::size_t bar_{};
  std::optional<std::size_t> last_buy_;
  std::optional<std::size_t> last_sell_;
};

} // namespace dagobert
