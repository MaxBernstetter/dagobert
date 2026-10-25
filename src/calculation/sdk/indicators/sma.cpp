#include "sma.hpp"

#include <cstdlib>
#include <iostream>

namespace dagobert {
namespace {

template <typename Series>
Series apply_series(const Series &input, std::size_t period) {
  Series output;
  const std::size_t count = input.samples.size();
  output.samples.assign(count, 0.0);
  output.ready.assign(count, 0);

  if (count < period) {
    return output;
  }

  double sum = 0.0;
  for (std::size_t i = 0; i < count; ++i) {
    sum += input.samples[i];
    if (i >= period) {
      sum -= input.samples[i - period];
    }
    if (i + 1U < period) {
      continue;
    }
    output.ready[i] = input.ready[i];
    if (output.ready[i]) {
      output.samples[i] = sum / static_cast<double>(period);
    }
  }
  return output;
}

} // namespace

SMA::SMA(std::size_t period_bars) : period_(period_bars) {
  if (period_bars == 0U) {
    std::cerr << "SMA period must be positive: " << period_bars << '\n';
    std::exit(2);
  }
}

CloseSeries SMA::apply_indicator(const CloseSeries &close) const {
  return apply_series(close, period_);
}

VolumeSeries SMA::apply_indicator(const VolumeSeries &volume) const {
  return apply_series(volume, period_);
}

std::optional<double> SMA::push(double sample) {
  window_.push_back(sample);
  window_sum_ += sample;
  if (window_.size() > period_) {
    window_sum_ -= window_.front();
    window_.pop_front();
  }
  if (window_.size() < period_) {
    return std::nullopt;
  }
  return window_sum_ / static_cast<double>(period_);
}

void SMA::reset() {
  window_.clear();
  window_sum_ = 0.0;
}

} // namespace dagobert
