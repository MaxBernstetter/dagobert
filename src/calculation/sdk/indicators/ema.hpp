#pragma once

#include "types.hpp"

#include <cstddef>
#include <optional>
#include <vector>

namespace dagobert {

class EMA {
public:
  explicit EMA(std::size_t period_bars);

  [[nodiscard]] std::size_t period() const { return period_; }

  [[nodiscard]] Column apply_indicator(const CloseSeries &close) const;

  [[nodiscard]] std::optional<double> push(double sample);
  void reset();

private:
  std::size_t period_;
  double multiplier_{0.0};
  bool initialized_{false};
  double prev_ema_{0.0};
  std::vector<double> seed_buffer_;
};

} // namespace dagobert
