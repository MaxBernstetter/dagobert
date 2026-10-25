#pragma once

#include "types.hpp"

#include <cstddef>
#include <optional>
#include <vector>

namespace dagobert {

struct MacdColumns {
  Column line;
  Column signal;
  Column histogram;
};

struct MacdSample {
  std::optional<double> line;
  std::optional<double> signal;
  std::optional<double> histogram;
};

class MACD {
public:
  MACD(std::size_t fast_period, std::size_t slow_period,
       std::size_t signal_period);

  [[nodiscard]] MacdColumns apply_indicator(const CloseSeries &close) const;

  [[nodiscard]] MacdSample push(double close);
  void reset();

private:
  std::size_t fast_period_;
  std::size_t slow_period_;
  std::size_t signal_period_;
  double fast_multiplier_{0.0};
  double slow_multiplier_{0.0};
  double signal_multiplier_{0.0};

  bool fast_initialized_{false};
  bool slow_initialized_{false};
  bool signal_initialized_{false};
  double fast_prev_ema_{0.0};
  double slow_prev_ema_{0.0};
  double signal_prev_ema_{0.0};
  std::vector<double> fast_seed_;
  std::vector<double> slow_seed_;
  std::size_t macd_valid_count_{0U};
  std::vector<double> macd_history_;
};

} // namespace dagobert
