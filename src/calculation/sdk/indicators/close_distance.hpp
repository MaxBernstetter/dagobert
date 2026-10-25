#pragma once

#include "sma.hpp"
#include "types.hpp"

#include <cstddef>
#include <optional>

namespace dagobert {

class CloseDistance {
public:
  explicit CloseDistance(std::size_t period_bars);

  [[nodiscard]] Column apply_indicator(const CloseSeries &close) const;

  [[nodiscard]] std::optional<double> push(double close);
  void reset();

private:
  SMA sma_;
};

} // namespace dagobert
