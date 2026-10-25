#pragma once

#include "types.hpp"

#include <cstddef>
#include <optional>
#include <vector>

namespace dagobert {

class ATR {
public:
  explicit ATR(std::size_t period_bars);

  [[nodiscard]] std::size_t period() const { return period_; }

  [[nodiscard]] Column apply_indicator(const std::vector<double> &high,
                                       const std::vector<double> &low,
                                       const CloseSeries &close) const;

  [[nodiscard]] std::optional<double> push(double high, double low,
                                           double close);
  void reset();

private:
  std::size_t period_;
  bool has_prev_close_{false};
  double prev_close_{0.0};
  bool initialized_{false};
  double prev_atr_{0.0};
  double tr_sum_{0.0};
  std::size_t tr_count_{0U};
};

} // namespace dagobert
