#pragma once

#include "consensus_gates.hpp"
#include "sizing.hpp"
#include "types.hpp"

#include <cstddef>
#include <optional>
#include <span>
#include <vector>

namespace dagobert {

class Position;

// vol_cut and pyramid only occur when a SizingConfig is set.
enum class ConsensusRule {
  consensus_buy,
  consensus_sell,
  atr_stop,
  vol_cut,
  pyramid
};

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
  // Inputs the sized path needs: the bar is usable and the ATR(24h) reading.
  bool valid{};
  std::optional<double> atr;
};

struct ConsensusSeries {
  std::vector<ConsensusGate> gate;
};

class ConsensusSignal {
public:
  // Without `sizing` the signal buys and sells the whole position. With it,
  // entries are sized from the ATR stop (stopAtrMult = kAtrStopMult) and each
  // bar does one of: atr_stop, consensus_sell, vol_cut, pyramid.
  ConsensusSignal() = default;
  explicit ConsensusSignal(std::optional<sizing::SizingConfig> sizing);

  [[nodiscard]] static ConsensusSeries evaluate(const ConsensusColumns &in);

  [[nodiscard]] static std::vector<Action>
  actions_from(Position &account, std::span<const Bar> bars,
               const ConsensusSeries &mask,
               std::optional<sizing::SizingConfig> sizing = std::nullopt);

  // Advances one bar; the caller executes the returned rule on its position.
  // This overload is for the unsized path (a sized signal reads equity 0 and
  // never enters).
  [[nodiscard]] std::optional<ConsensusRule> apply(const ConsensusSample &in,
                                                   bool flat);
  // Sized path: equity and open state come from the account.
  [[nodiscard]] std::optional<ConsensusRule> apply(const ConsensusSample &in,
                                                   const Position &account);

  void execute(Position &account, const Bar &bar, ConsensusRule rule);
  void reset();

private:
  struct Plan {
    double notional{};
    double stop{};
    double raise_by{};
    double qty{};
  };

  [[nodiscard]] std::optional<ConsensusRule>
  decide(const ConsensusGate &gate, double close, double equity, bool flat);
  [[nodiscard]] std::optional<ConsensusRule>
  decide_sized(const ConsensusGate &gate, double close, double equity,
               bool flat, std::size_t i);
  void buy_unit(Position &account, const Bar &bar, ConsensusRule rule);

  BaselineCross cross_;
  RsiVote rsi_vote_;
  std::size_t bar_{};
  std::optional<std::size_t> last_buy_;
  std::optional<std::size_t> last_sell_;
  std::optional<sizing::SizingConfig> sizing_;
  std::vector<sizing::Unit> units_;
  Plan plan_;
};

} // namespace dagobert
