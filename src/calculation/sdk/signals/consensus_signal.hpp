#pragma once

#include "consensus_gates.hpp"
#include "sizing.hpp"
#include "types.hpp"

#include <climits>
#include <cstddef>
#include <cstdint>
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
  // Per bar: 1 when the bar directly follows the previous one. Null means
  // every bar does. The Go playbook does not trade a bar after a gap.
  const std::vector<char> *prior{nullptr};
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
  // False when a bar is missing before this one: no decision on this bar.
  bool prior{true};
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
  // entries are sized from the ATR stop (stopAtrMult = params.atr_stop_mult)
  // and each bar does one of: atr_stop, consensus_sell, vol_cut, pyramid.
  ConsensusSignal() = default;
  explicit ConsensusSignal(std::optional<sizing::SizingConfig> sizing,
                           const ConsensusParams &params = {});

  [[nodiscard]] static ConsensusSeries
  evaluate(const ConsensusColumns &in, const ConsensusParams &params = {});

  [[nodiscard]] static std::vector<Action>
  actions_from(Position &account, std::span<const Bar> bars,
               const ConsensusSeries &mask,
               std::optional<sizing::SizingConfig> sizing = std::nullopt,
               const ConsensusParams &params = {},
               std::int64_t trade_from_ms = INT64_MIN);

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
    double equity{};
    double atr{};
  };

  [[nodiscard]] std::optional<ConsensusRule>
  decide(const ConsensusGate &gate, double close, double equity, bool flat);
  [[nodiscard]] std::optional<ConsensusRule>
  decide_sized(const ConsensusGate &gate, double close, double equity,
               bool flat, std::size_t i);
  void buy_unit(Position &account, const Bar &bar, ConsensusRule rule);

  ConsensusParams params_;
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
