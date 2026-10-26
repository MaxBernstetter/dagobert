#include "consensus_features.hpp"

#include <algorithm>
#include <cmath>
#include <vector>

namespace dagobert {
namespace {

// EMA over the last `period` of closes[0, end), seeded with the first of them.
[[nodiscard]] double ema_tail(const std::vector<double> &closes,
                              std::size_t end, std::size_t period) {
  if (end == 0 || period == 0) {
    return 0.0;
  }
  period = std::min(period, end);
  const std::size_t first = end - period;
  if (period == 1) {
    return closes[first];
  }
  const double alpha = 2.0 / static_cast<double>(period + 1);
  double out = closes[first];
  for (std::size_t i = first + 1; i < end; ++i) {
    out = alpha * closes[i] + (1.0 - alpha) * out;
  }
  return out;
}

[[nodiscard]] double rsi_of(const std::vector<double> &closes) {
  double gain = 0.0;
  double loss = 0.0;
  for (std::size_t i = 1; i < closes.size(); ++i) {
    const double delta = closes[i] - closes[i - 1];
    if (delta > 0.0) {
      gain += delta;
    } else if (delta < 0.0) {
      loss += -delta;
    }
  }
  const auto n = static_cast<double>(closes.size() - 1);
  gain /= n;
  loss /= n;
  if (gain == 0.0 && loss == 0.0) {
    return 50.0;
  }
  if (loss == 0.0 && gain > 0.0) {
    return 100.0;
  }
  return 100.0 - 100.0 / (1.0 + gain / loss);
}

} // namespace

ConsensusFeatures::ConsensusFeatures(const ConsensusWindows &windows)
    : w_(windows), cap_(std::max({windows.macro, windows.regime, windows.signal,
                                  windows.atr + 1, windows.volume_mean})) {}

double ConsensusFeatures::tail_mean_close(std::size_t n) const {
  double sum = 0.0;
  for (std::size_t i = history_.size() - n; i < history_.size(); ++i) {
    sum += history_[i].close;
  }
  return sum / static_cast<double>(n);
}

std::optional<double> ConsensusFeatures::atr() const {
  const std::size_t n = w_.atr;
  if (history_.size() < 2 || history_.size() < n + 1) {
    return std::nullopt;
  }
  double sum = 0.0;
  for (std::size_t i = history_.size() - n; i < history_.size(); ++i) {
    const Entry &cur = history_[i];
    const double prev_close = history_[i - 1].close;
    const double tr =
        std::max(cur.high - cur.low, std::max(std::abs(cur.high - prev_close),
                                              std::abs(cur.low - prev_close)));
    sum += tr;
  }
  return sum / static_cast<double>(n);
}

std::optional<double> ConsensusFeatures::histogram() const {
  const std::size_t len = history_.size();
  if (len < w_.macd_slow + w_.macd_signal) {
    return std::nullopt;
  }
  std::vector<double> closes;
  closes.reserve(len);
  for (const Entry &e : history_) {
    closes.push_back(e.close);
  }
  // The signal EMA reads only the last macd_signal MACD values.
  std::vector<double> line;
  for (std::size_t j = len - w_.macd_signal; j < len; ++j) {
    line.push_back(ema_tail(closes, j + 1, w_.macd_fast) -
                   ema_tail(closes, j + 1, w_.macd_slow));
  }
  return line.back() - ema_tail(line, line.size(), w_.macd_signal);
}

ConsensusSample ConsensusFeatures::push(const Bar &bar) {
  history_.push_back({.high = bar.high,
                      .low = bar.low,
                      .close = bar.close,
                      .volume = bar.volume});
  if (history_.size() > cap_) {
    history_.pop_front();
  }
  ConsensusSample s;
  s.close = bar.close;
  s.volume = bar.volume;
  s.prior = has_last_ && bar.open_ms - last_open_ms_ == bar.duration_ms;
  last_open_ms_ = bar.open_ms;
  has_last_ = true;

  const auto sma = [this](std::size_t n) -> std::optional<double> {
    if (history_.size() < n) {
      return std::nullopt;
    }
    return tail_mean_close(n);
  };
  s.signal = sma(w_.signal);
  s.regime = sma(w_.regime);
  s.macro = sma(w_.macro);
  s.atr = atr();
  s.hist = histogram();
  if (bar.segment_close.size() >= 2) {
    s.rsi = rsi_of(bar.segment_close);
  }
  if (history_.size() >= w_.volume_mean) {
    double sum = 0.0;
    for (std::size_t i = history_.size() - w_.volume_mean; i < history_.size();
         ++i) {
      sum += history_[i].volume;
    }
    s.vol_mean = sum / static_cast<double>(w_.volume_mean);
  }
  return s;
}

void ConsensusFeatures::reset() {
  history_.clear();
  has_last_ = false;
}

} // namespace dagobert
