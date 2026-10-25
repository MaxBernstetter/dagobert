#pragma once

#include "sizing.hpp"
#include "types.hpp"

#include <cstddef>
#include <optional>
#include <span>
#include <utility>
#include <vector>

namespace dagobert {

class Position;

enum class SmaCrossRule {
  golden_cross,
  death_cross,
  atr_stop,
  vol_cut,
  pyramid
};

struct SmaCrossSeries {
  std::vector<std::optional<SmaCrossRule>> rule;
};

class SmaCrossSignal {
public:
  SmaCrossSignal() = default;
  // With a sizing config, step_sized() runs the position-sizing flow.
  explicit SmaCrossSignal(sizing::SizingConfig sizing)
      : sizing_(std::move(sizing)) {}

  [[nodiscard]] bool sized() const { return sizing_.has_value(); }

  [[nodiscard]] static SmaCrossSeries evaluate(const CloseSeries &fast,
                                               const CloseSeries &slow);

  [[nodiscard]] static std::vector<Action>
  actions_from(Position &account, std::span<const Bar> bars,
               const SmaCrossSeries &mask);

  [[nodiscard]] std::optional<SmaCrossRule>
  apply(std::optional<double> fast, std::optional<double> slow, bool flat);

  void execute(Position &account, const Bar &bar, SmaCrossRule rule);

  // Sized flow, one bar at a time (backtest and live share it). Entry on a
  // golden cross; while long one action per bar, in order: atr_stop,
  // death_cross, vol_cut, pyramid. Requires sized().
  void step_sized(Position &account, const Bar &bar, std::optional<double> fast,
                  std::optional<double> slow, std::optional<double> atr);

  void reset();

private:
  std::optional<bool> prev_above_;
  std::optional<sizing::SizingConfig> sizing_;
  std::vector<sizing::Unit> units_;
};

} // namespace dagobert
