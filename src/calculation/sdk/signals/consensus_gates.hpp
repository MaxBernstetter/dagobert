#pragma once

#include "types.hpp"

#include <cstddef>
#include <optional>
#include <vector>

namespace dagobert {

enum class Cross { none, up, down };
enum class Vote { bear = -1, neutral = 0, bull = 1 };

// Thresholds the gates and the cooldowns read. The member defaults are the Go
// signal-consensus playbook defaults.
struct ConsensusParams {
  double rsi_bull{52.0};
  double rsi_bear{48.0};
  double vol_ratio_bull{1.10};
  double vol_ratio_bear{0.85};
  double atr_stop_mult{2.5};
  std::size_t min_bars_flat_after_sell{8};
  std::size_t min_bars_long_before_sell{4};
};

// Close crosses a baseline: prev_close <= prev_baseline && close > baseline.
class BaselineCross {
public:
  [[nodiscard]] static std::vector<Cross> evaluate(const CloseSeries &close,
                                                   const Column &baseline);
  [[nodiscard]] Cross apply(double close, std::optional<double> baseline);
  void reset();

private:
  std::optional<double> prev_close_;
  std::optional<double> prev_baseline_;
};

// Close above both trend averages and regime above macro.
class HigherTrend {
public:
  [[nodiscard]] static std::vector<char>
  evaluate(const CloseSeries &close, const Column &regime, const Column &macro);
  [[nodiscard]] static bool apply(double close, std::optional<double> regime,
                                  std::optional<double> macro);
};

class RsiVote {
public:
  explicit RsiVote(const ConsensusParams &params = {}) : params_(params) {}
  [[nodiscard]] static std::vector<Vote>
  evaluate(const Column &rsi, const ConsensusParams &params = {});
  [[nodiscard]] Vote apply(std::optional<double> rsi);
  void reset();

private:
  ConsensusParams params_;
  std::optional<double> prev_;
};

// Volume against its own mean; takes volume series only, never closes.
class VolumeRatio {
public:
  [[nodiscard]] static std::vector<Vote>
  evaluate(const VolumeSeries &volume, const Column &mean,
           const ConsensusParams &params = {});
  [[nodiscard]] static Vote apply(double volume, std::optional<double> mean,
                                  const ConsensusParams &params = {});
};

// Histogram above zero; falls back to the momentum vote without a histogram.
class MacdSign {
public:
  [[nodiscard]] static std::vector<char>
  evaluate(const Column &histogram, const std::vector<Vote> &momentum);
  [[nodiscard]] static bool apply(std::optional<double> histogram,
                                  Vote momentum);
};

// Close below baseline - atr_stop_mult * ATR.
class AtrStop {
public:
  [[nodiscard]] static std::vector<char>
  evaluate(const CloseSeries &close, const Column &baseline, const Column &atr,
           const ConsensusParams &params = {});
  [[nodiscard]] static bool apply(double close, std::optional<double> baseline,
                                  std::optional<double> atr,
                                  const ConsensusParams &params = {});
};

} // namespace dagobert
