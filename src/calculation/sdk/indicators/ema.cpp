#include "ema.hpp"

#include "exit_policy.hpp"

#include <iostream>

namespace dagobert {
namespace {

Column apply_ema(const CloseSeries &close, std::size_t period,
                 double multiplier) {
  Column output;
  const std::size_t count = close.samples.size();
  output.samples.assign(count, 0.0);
  output.ready.assign(count, 0);

  bool initialized = false;
  double prev_ema = 0.0;
  for (std::size_t i = 0; i < count; ++i) {
    if (i + 1U < period) {
      continue;
    }
    if (!initialized) {
      double sum = 0.0;
      for (std::size_t j = i + 1U - period; j <= i; ++j) {
        sum += close.samples[j];
      }
      prev_ema = sum / static_cast<double>(period);
      initialized = true;
      output.ready[i] = close.ready[i];
      if (output.ready[i]) {
        output.samples[i] = prev_ema;
      }
      continue;
    }
    const double current = close.samples[i];
    prev_ema = (current - prev_ema) * multiplier + prev_ema;
    output.ready[i] = close.ready[i];
    if (output.ready[i]) {
      output.samples[i] = prev_ema;
    }
  }
  return output;
}

} // namespace

EMA::EMA(std::size_t period_bars) : period_(period_bars) {
  if (period_bars == 0U) {
    std::cerr << "EMA period must be positive: " << period_bars << '\n';
    exit_config();
  }
  multiplier_ = 2.0 / (static_cast<double>(period_) + 1.0);
}

Column EMA::apply_indicator(const CloseSeries &close) const {
  return apply_ema(close, period_, multiplier_);
}

std::optional<double> EMA::push(double sample) {
  if (!initialized_) {
    seed_buffer_.push_back(sample);
    if (seed_buffer_.size() < period_) {
      return std::nullopt;
    }
    double sum = 0.0;
    for (const double value : seed_buffer_) {
      sum += value;
    }
    prev_ema_ = sum / static_cast<double>(period_);
    initialized_ = true;
    seed_buffer_.clear();
    return prev_ema_;
  }
  prev_ema_ = (sample - prev_ema_) * multiplier_ + prev_ema_;
  return prev_ema_;
}

void EMA::reset() {
  initialized_ = false;
  prev_ema_ = 0.0;
  seed_buffer_.clear();
}

} // namespace dagobert
