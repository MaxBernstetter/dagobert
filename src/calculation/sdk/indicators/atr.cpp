#include "atr.hpp"

#include <algorithm>
#include <cmath>
#include "exit_policy.hpp"

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

  // Simple mean of the last `period` true ranges (the Go technical-indicator
  // job's definition; there is no Wilder smoothing).
  double tr_sum = 0.0;
  for (std::size_t i = 1; i < count; ++i) {
    tr_sum += true_range(high[i], low[i], close.samples[i - 1U]);
    if (i > period) {
      const std::size_t old = i - period;
      tr_sum -= true_range(high[old], low[old], close.samples[old - 1U]);
    }
    if (i < period) {
      continue;
    }
    output.ready[i] = close.ready[i];
    if (output.ready[i]) {
      output.samples[i] = tr_sum / static_cast<double>(period);
    }
  }
  return output;
}

} // namespace

ATR::ATR(std::size_t period_bars) : period_(period_bars) {
  if (period_bars == 0U) {
    std::cerr << "ATR period must be positive: " << period_bars << '\n';
    exit_config();
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

  trs_.push_back(tr);
  if (trs_.size() > period_) {
    trs_.pop_front();
  }
  if (trs_.size() < period_) {
    return std::nullopt;
  }
  double sum = 0.0;
  for (const double v : trs_) {
    sum += v;
  }
  return sum / static_cast<double>(period_);
}

void ATR::reset() {
  has_prev_close_ = false;
  prev_close_ = 0.0;
  trs_.clear();
}

} // namespace dagobert
