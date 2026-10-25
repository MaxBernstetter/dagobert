#pragma once

#include "types.hpp"

#include <optional>
#include <vector>

namespace dagobert {

enum class Cross { none, up, down };
enum class Vote { bear = -1, neutral = 0, bull = 1 };

inline constexpr double kRsiBull = 52.0;
inline constexpr double kRsiBear = 48.0;
inline constexpr double kVolRatioBull = 1.10;
inline constexpr double kVolRatioBear = 0.85;
inline constexpr double kAtrStopMult = 2.5;

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
  [[nodiscard]] static std::vector<Vote> evaluate(const Column &rsi);
  [[nodiscard]] Vote apply(std::optional<double> rsi);
  void reset();

private:
  std::optional<double> prev_;
};

// Volume against its own mean; takes volume series only, never closes.
class VolumeRatio {
public:
  [[nodiscard]] static std::vector<Vote> evaluate(const VolumeSeries &volume,
                                                  const Column &mean);
  [[nodiscard]] static Vote apply(double volume, std::optional<double> mean);
};

// Histogram above zero; falls back to the momentum vote without a histogram.
class MacdSign {
public:
  [[nodiscard]] static std::vector<char>
  evaluate(const Column &histogram, const std::vector<Vote> &momentum);
  [[nodiscard]] static bool apply(std::optional<double> histogram,
                                  Vote momentum);
};

// Close below baseline - mult * ATR.
class AtrStop {
public:
  [[nodiscard]] static std::vector<char>
  evaluate(const CloseSeries &close, const Column &baseline, const Column &atr);
  [[nodiscard]] static bool apply(double close, std::optional<double> baseline,
                                  std::optional<double> atr);
};

} // namespace dagobert
