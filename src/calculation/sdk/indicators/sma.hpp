#pragma once

#include "types.hpp" // from data-models (via target include directories)

#include <cstddef>
#include <deque>
#include <optional>

namespace dagobert {

class SMA {
public:
  explicit SMA(std::size_t period_bars);

  [[nodiscard]] std::size_t period() const { return period_; }

  [[nodiscard]] CloseSeries apply_indicator(const CloseSeries &close) const;
  [[nodiscard]] VolumeSeries apply_indicator(const VolumeSeries &volume) const;

  [[nodiscard]] std::optional<double> push(double sample);
  void reset();

private:
  std::size_t period_;
  std::deque<double> window_;
  double window_sum_{0.0};
};

} // namespace dagobert
