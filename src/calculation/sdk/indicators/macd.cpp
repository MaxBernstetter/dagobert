#include "macd.hpp"

#include <cstdlib>
#include <iostream>
#include <numeric>

namespace dagobert {
namespace {

double seed_sma(const std::vector<double> &samples, std::size_t end_index,
                std::size_t period) {
  double sum = 0.0;
  for (std::size_t i = 0; i < period; ++i) {
    sum += samples[end_index - i];
  }
  return sum / static_cast<double>(period);
}

double advance_ema(double current, double prev, double multiplier) {
  return (current - prev) * multiplier + prev;
}

MacdColumns apply_macd(const CloseSeries &close, std::size_t fast_period,
                       std::size_t slow_period, std::size_t signal_period,
                       double fast_multiplier, double slow_multiplier,
                       double signal_multiplier) {
  MacdColumns out;
  const std::size_t count = close.samples.size();
  out.line.samples.assign(count, 0.0);
  out.line.ready.assign(count, 0);
  out.signal.samples.assign(count, 0.0);
  out.signal.ready.assign(count, 0);
  out.histogram.samples.assign(count, 0.0);
  out.histogram.ready.assign(count, 0);

  bool fast_initialized = false;
  bool slow_initialized = false;
  bool signal_initialized = false;
  double fast_prev = 0.0;
  double slow_prev = 0.0;
  double signal_prev = 0.0;
  std::size_t macd_valid_count = 0U;
  std::vector<double> macd_history;

  for (std::size_t i = 0; i < count; ++i) {
    const double sample = close.samples[i];

    if (i + 1U < fast_period) {
      continue;
    }
    if (!fast_initialized) {
      fast_prev = seed_sma(close.samples, i, fast_period);
      fast_initialized = true;
    } else {
      fast_prev = advance_ema(sample, fast_prev, fast_multiplier);
    }

    if (i + 1U < slow_period) {
      continue;
    }
    if (!slow_initialized) {
      slow_prev = seed_sma(close.samples, i, slow_period);
      slow_initialized = true;
    } else {
      slow_prev = advance_ema(sample, slow_prev, slow_multiplier);
    }

    const double macd_value = fast_prev - slow_prev;
    out.line.ready[i] = close.ready[i];
    if (out.line.ready[i]) {
      out.line.samples[i] = macd_value;
    }
    macd_history.push_back(macd_value);
    ++macd_valid_count;

    if (macd_valid_count < signal_period) {
      continue;
    }

    if (!signal_initialized) {
      double signal_sum = 0.0;
      for (std::size_t j = 0; j < signal_period; ++j) {
        signal_sum += macd_history[macd_history.size() - signal_period + j];
      }
      signal_prev = signal_sum / static_cast<double>(signal_period);
      signal_initialized = true;
    } else {
      signal_prev = advance_ema(macd_value, signal_prev, signal_multiplier);
    }

    const double histogram = macd_value - signal_prev;
    out.signal.ready[i] = close.ready[i];
    out.histogram.ready[i] = close.ready[i];
    if (out.signal.ready[i]) {
      out.signal.samples[i] = signal_prev;
      out.histogram.samples[i] = histogram;
    }
  }
  return out;
}

} // namespace

MACD::MACD(std::size_t fast_period, std::size_t slow_period,
           std::size_t signal_period)
    : fast_period_(fast_period == 0U ? 1U : fast_period),
      slow_period_(slow_period == 0U ? 1U : slow_period),
      signal_period_(signal_period == 0U ? 1U : signal_period) {
  if (fast_period == 0U || slow_period == 0U || signal_period == 0U) {
    std::cerr << "MACD periods must be positive\n";
    std::exit(2);
  }
  fast_multiplier_ = 2.0 / (static_cast<double>(fast_period_) + 1.0);
  slow_multiplier_ = 2.0 / (static_cast<double>(slow_period_) + 1.0);
  signal_multiplier_ = 2.0 / (static_cast<double>(signal_period_) + 1.0);
}

MacdColumns MACD::apply_indicator(const CloseSeries &close) const {
  return apply_macd(close, fast_period_, slow_period_, signal_period_,
                    fast_multiplier_, slow_multiplier_, signal_multiplier_);
}

MacdSample MACD::push(double close) {
  MacdSample out;

  if (!fast_initialized_) {
    fast_seed_.push_back(close);
    if (fast_seed_.size() >= fast_period_) {
      fast_prev_ema_ =
          std::accumulate(fast_seed_.begin(), fast_seed_.end(), 0.0) /
          static_cast<double>(fast_period_);
      fast_initialized_ = true;
      fast_seed_.clear();
    }
  } else {
    fast_prev_ema_ = advance_ema(close, fast_prev_ema_, fast_multiplier_);
  }

  if (!slow_initialized_) {
    slow_seed_.push_back(close);
    if (slow_seed_.size() < slow_period_) {
      return out;
    }
    slow_prev_ema_ =
        std::accumulate(slow_seed_.begin(), slow_seed_.end(), 0.0) /
        static_cast<double>(slow_period_);
    slow_initialized_ = true;
    slow_seed_.clear();
  } else {
    slow_prev_ema_ = advance_ema(close, slow_prev_ema_, slow_multiplier_);
  }

  if (!fast_initialized_) {
    return out;
  }

  const double macd_value = fast_prev_ema_ - slow_prev_ema_;
  out.line = macd_value;
  macd_history_.push_back(macd_value);
  ++macd_valid_count_;

  if (macd_valid_count_ < signal_period_) {
    return out;
  }

  if (!signal_initialized_) {
    double signal_sum = 0.0;
    for (std::size_t j = 0; j < signal_period_; ++j) {
      signal_sum += macd_history_[macd_history_.size() - signal_period_ + j];
    }
    signal_prev_ema_ = signal_sum / static_cast<double>(signal_period_);
    signal_initialized_ = true;
  } else {
    signal_prev_ema_ =
        advance_ema(macd_value, signal_prev_ema_, signal_multiplier_);
  }

  out.signal = signal_prev_ema_;
  out.histogram = macd_value - signal_prev_ema_;
  return out;
}

void MACD::reset() {
  fast_initialized_ = false;
  slow_initialized_ = false;
  signal_initialized_ = false;
  fast_prev_ema_ = 0.0;
  slow_prev_ema_ = 0.0;
  signal_prev_ema_ = 0.0;
  fast_seed_.clear();
  slow_seed_.clear();
  macd_valid_count_ = 0U;
  macd_history_.clear();
}

} // namespace dagobert
