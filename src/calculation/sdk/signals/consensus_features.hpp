#pragma once

#include "consensus_signal.hpp"

#include <cstddef>
#include <deque>

namespace dagobert {

// Window lengths in bars. The consensus playbook fixes them in hours; the
// caller converts with bars_for.
struct ConsensusWindows {
  std::size_t signal{4};      // SMA 12h
  std::size_t regime{8};      // SMA 24h
  std::size_t macro{16};      // SMA 48h
  std::size_t atr{8};         // ATR 24h
  std::size_t volume_mean{1}; // volume-mean 3h
  std::size_t macd_fast{4};   // 12h
  std::size_t macd_slow{8};   // 24h
  std::size_t macd_signal{3}; // 9h
};

// The indicator series the Go signal-consensus playbook reads, computed with
// the definitions of the technical-indicator job (calculation-technical-
// indicator/internal/indicators), not the textbook ones:
//   SMA     mean of the last n closes.
//   ATR     mean of the last n true ranges (the Wilder loop never smooths).
//   RSI     simple mean gain/loss over the sub-bar closes of the current bar
//           (Bar::segment_close); not ready with fewer than two.
//   volume  mean of the last n bar volumes (n = 1 on 3h bars: the bar itself).
//   MACD    histogram of EMAs that seed on the last N closes only.
// Only the last `macro` bars are kept, like the job's trailing window, so a
// value is ready when the kept history is long enough.
class ConsensusFeatures {
public:
  explicit ConsensusFeatures(const ConsensusWindows &windows);

  // Adds the next bar and returns the series at it. `prior` is false when the
  // bar does not directly follow the previous one (a missing bar in between).
  [[nodiscard]] ConsensusSample push(const Bar &bar);
  void reset();

private:
  struct Entry {
    double high{};
    double low{};
    double close{};
    double volume{};
  };

  [[nodiscard]] double tail_mean_close(std::size_t n) const;
  [[nodiscard]] std::optional<double> atr() const;
  [[nodiscard]] std::optional<double> histogram() const;

  ConsensusWindows w_;
  std::size_t cap_;
  std::deque<Entry> history_;
  std::int64_t last_open_ms_{};
  bool has_last_{false};
};

} // namespace dagobert
