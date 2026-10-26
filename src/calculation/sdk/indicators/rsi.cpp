#include "rsi.hpp"

#include <algorithm>
#include <cmath>
#include "exit_policy.hpp"

#include <cstdlib>
#include <iostream>

namespace dagobert {
namespace {

void push_gain_loss(double change, std::deque<double> &gains,
                    std::deque<double> &losses) {
  gains.push_back(std::max(change, 0.0));
  losses.push_back(std::max(-change, 0.0));
}

double mean_deque(const std::deque<double> &values) {
  double sum = 0.0;
  for (const double value : values) {
    sum += value;
  }
  return sum / static_cast<double>(values.size());
}

std::optional<double> rsi_value(double avg_gain, double avg_loss) {
  if (avg_loss == 0.0) {
    if (avg_gain == 0.0) {
      return std::nullopt;
    }
    return 100.0;
  }
  const double rs = avg_gain / avg_loss;
  return 100.0 - (100.0 / (1.0 + rs));
}

Column apply_rsi(const CloseSeries &close, std::size_t period) {
  Column output;
  const std::size_t count = close.samples.size();
  output.samples.assign(count, 0.0);
  output.ready.assign(count, 0);

  if (count < 2U) {
    return output;
  }

  std::deque<double> gains;
  std::deque<double> losses;
  bool initialized = false;
  double avg_gain = 0.0;
  double avg_loss = 0.0;
  const double period_d = static_cast<double>(period);

  for (std::size_t i = 1; i < count; ++i) {
    const double change = close.samples[i] - close.samples[i - 1U];
    push_gain_loss(change, gains, losses);

    if (gains.size() < period) {
      continue;
    }

    if (!initialized) {
      avg_gain = mean_deque(gains);
      avg_loss = mean_deque(losses);
      initialized = true;
    } else {
      const double gain = gains.back();
      const double loss = losses.back();
      avg_gain = (avg_gain * (period_d - 1.0) + gain) / period_d;
      avg_loss = (avg_loss * (period_d - 1.0) + loss) / period_d;
    }

    const auto rsi = rsi_value(avg_gain, avg_loss);
    if (!rsi.has_value()) {
      continue;
    }
    output.ready[i] = close.ready[i];
    if (output.ready[i]) {
      output.samples[i] = *rsi;
    }
  }
  return output;
}

} // namespace

RSI::RSI(std::size_t period_bars) : period_(period_bars) {
  if (period_bars == 0U) {
    std::cerr << "RSI period must be positive: " << period_bars << '\n';
    exit_config();
  }
}

Column RSI::apply_indicator(const CloseSeries &close) const {
  return apply_rsi(close, period_);
}

std::optional<double> RSI::rsi_from_averages(double avg_gain,
                                             double avg_loss) const {
  return rsi_value(avg_gain, avg_loss);
}

std::optional<double> RSI::push(double sample) {
  if (!has_prev_close_) {
    prev_close_ = sample;
    has_prev_close_ = true;
    return std::nullopt;
  }

  const double change = sample - prev_close_;
  prev_close_ = sample;
  const double gain = std::max(change, 0.0);
  const double loss = std::max(-change, 0.0);

  if (!initialized_) {
    seed_gains_.push_back(gain);
    seed_losses_.push_back(loss);
    if (seed_gains_.size() < period_) {
      return std::nullopt;
    }
    avg_gain_ = mean_deque(seed_gains_);
    avg_loss_ = mean_deque(seed_losses_);
    initialized_ = true;
    seed_gains_.clear();
    seed_losses_.clear();
    return rsi_from_averages(avg_gain_, avg_loss_);
  }

  const double period_d = static_cast<double>(period_);
  avg_gain_ = (avg_gain_ * (period_d - 1.0) + gain) / period_d;
  avg_loss_ = (avg_loss_ * (period_d - 1.0) + loss) / period_d;
  return rsi_from_averages(avg_gain_, avg_loss_);
}

void RSI::reset() {
  has_prev_close_ = false;
  prev_close_ = 0.0;
  initialized_ = false;
  avg_gain_ = 0.0;
  avg_loss_ = 0.0;
  seed_gains_.clear();
  seed_losses_.clear();
}

} // namespace dagobert
