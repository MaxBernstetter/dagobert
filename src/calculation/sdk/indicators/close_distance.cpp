#include "close_distance.hpp"

namespace dagobert {

CloseDistance::CloseDistance(std::size_t period_bars) : sma_{period_bars} {}

Column CloseDistance::apply_indicator(const CloseSeries &close) const {
  const CloseSeries mean = sma_.apply_indicator(close);
  Column output;
  const std::size_t count = close.samples.size();
  output.samples.assign(count, 0.0);
  output.ready.assign(count, 0);
  for (std::size_t i = 0; i < count; ++i) {
    if (!mean.ready[i] || mean.samples[i] == 0.0) {
      continue;
    }
    output.ready[i] = close.ready[i];
    if (output.ready[i]) {
      output.samples[i] =
          (close.samples[i] - mean.samples[i]) / mean.samples[i];
    }
  }
  return output;
}

std::optional<double> CloseDistance::push(double close) {
  const auto mean = sma_.push(close);
  if (!mean.has_value() || *mean == 0.0) {
    return std::nullopt;
  }
  return (close - *mean) / *mean;
}

void CloseDistance::reset() { sma_.reset(); }

} // namespace dagobert
