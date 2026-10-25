#pragma once

#include "types.hpp"

#include <cstddef>
#include <deque>
#include <optional>

namespace dagobert {

class RSI {
public:
  explicit RSI(std::size_t period_bars);

  [[nodiscard]] std::size_t period() const { return period_; }

  [[nodiscard]] Column apply_indicator(const CloseSeries &close) const;

  [[nodiscard]] std::optional<double> push(double sample);
  void reset();

private:
  [[nodiscard]] std::optional<double> rsi_from_averages(double avg_gain,
                                                        double avg_loss) const;

  std::size_t period_;
  bool has_prev_close_{false};
  double prev_close_{0.0};
  bool initialized_{false};
  double avg_gain_{0.0};
  double avg_loss_{0.0};
  std::deque<double> seed_gains_;
  std::deque<double> seed_losses_;
};

} // namespace dagobert
