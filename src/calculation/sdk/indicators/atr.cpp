#include "atr.hpp"

#include <algorithm>
#include <cmath>
#include <cstdlib>
#include <iostream>

namespace dagobert {
namespace {

double true_range(double high, double low, double prev_close) {
  const double high_low = high - low;
  const double high_prev = std::abs(high - prev_close);
  const double low_prev = std::abs(low - prev_close);
  return std::max({high_low, high_prev, low_prev});
}

Column apply_atr(const std::vector<double> &high,
                 const std::vector<double> &low, const CloseSeries &close,
                 std::size_t period) {
  Column output;
  const std::size_t count = close.samples.size();
  output.samples.assign(count, 0.0);
  output.ready.assign(count, 0);
  if (count < 2U || high.size() != count || low.size() != count) {
    return output;
  }

  bool initialized = false;
  double prev_atr = 0.0;
  double tr_sum = 0.0;
  std::size_t tr_count = 0U;
  const double period_d = static_cast<double>(period);

  for (std::size_t i = 1; i < count; ++i) {
    const double tr = true_range(high[i], low[i], close.samples[i - 1U]);
    if (!initialized) {
      tr_sum += tr;
      ++tr_count;
      if (tr_count < period) {
        continue;
      }
      prev_atr = tr_sum / period_d;
      initialized = true;
      output.ready[i] = close.ready[i];
      if (output.ready[i]) {
        output.samples[i] = prev_atr;
      }
      continue;
    }
    prev_atr = ((prev_atr * (period_d - 1.0)) + tr) / period_d;
    output.ready[i] = close.ready[i];
    if (output.ready[i]) {
      output.samples[i] = prev_atr;
    }
  }
  return output;
}

} // namespace

ATR::ATR(std::size_t period_bars) : period_(period_bars) {
  if (period_bars == 0U) {
    std::cerr << "ATR period must be positive: " << period_bars << '\n';
    std::exit(2);
  }
}

Column ATR::apply_indicator(const std::vector<double> &high,
                            const std::vector<double> &low,
                            const CloseSeries &close) const {
  return apply_atr(high, low, close, period_);
}

std::optional<double> ATR::push(double high, double low, double close) {
  if (!has_prev_close_) {
    prev_close_ = close;
    has_prev_close_ = true;
    return std::nullopt;
  }
  const double tr = true_range(high, low, prev_close_);
  prev_close_ = close;

  if (!initialized_) {
    tr_sum_ += tr;
    ++tr_count_;
    if (tr_count_ < period_) {
      return std::nullopt;
    }
    prev_atr_ = tr_sum_ / static_cast<double>(period_);
    initialized_ = true;
    return prev_atr_;
  }

  const double period_d = static_cast<double>(period_);
  prev_atr_ = ((prev_atr_ * (period_d - 1.0)) + tr) / period_d;
  return prev_atr_;
}

void ATR::reset() {
  has_prev_close_ = false;
  prev_close_ = 0.0;
  initialized_ = false;
  prev_atr_ = 0.0;
  tr_sum_ = 0.0;
  tr_count_ = 0U;
}

} // namespace dagobert
