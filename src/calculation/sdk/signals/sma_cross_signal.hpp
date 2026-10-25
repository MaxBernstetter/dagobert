#pragma once

#include "types.hpp"

#include <cstddef>
#include <optional>
#include <span>
#include <vector>

namespace dagobert {

class Position;

enum class SmaCrossRule { golden_cross, death_cross };

struct SmaCrossSeries {
  std::vector<std::optional<SmaCrossRule>> rule;
};

class SmaCrossSignal {
public:
  [[nodiscard]] static SmaCrossSeries evaluate(const CloseSeries& fast,
                                               const CloseSeries& slow);

  [[nodiscard]] static std::vector<Action>
  actions_from(Position& account, std::span<const Bar> bars,
               const SmaCrossSeries& mask);

  [[nodiscard]] std::optional<SmaCrossRule>
  apply(std::optional<double> fast, std::optional<double> slow, bool flat);

  void execute(Position& account, const Bar& bar, SmaCrossRule rule);
  void reset();

private:
  std::optional<bool> prev_above_;
};

} // namespace dagobert
