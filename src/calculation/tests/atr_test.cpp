#include <gtest/gtest.h>

#include "atr.hpp"

#include <cmath>

namespace dagobert {
namespace {

CloseSeries fixture_close() {
  CloseSeries close;
  close.samples = {10.0, 11.0, 12.0, 11.5, 13.0, 12.0, 14.0, 13.5, 15.0, 14.5};
  close.ready.assign(close.samples.size(), 1);
  return close;
}

std::vector<double> fixture_high(const CloseSeries& close) {
  std::vector<double> high;
  for (const double c : close.samples) {
    high.push_back(c + 0.5);
  }
  return high;
}

std::vector<double> fixture_low(const CloseSeries& close) {
  std::vector<double> low;
  for (const double c : close.samples) {
    low.push_back(c - 0.5);
  }
  return low;
}

void construct_zero_atr() { ATR{0U}; }

[[nodiscard]] bool near_relative(double expected, double actual) {
  if (expected == 0.0) {
    return std::abs(actual) < 1e-12;
  }
  return std::abs((actual - expected) / expected) < 1e-12;
}

} // namespace

TEST(ATR, PeriodZeroExits) {
  EXPECT_EXIT(construct_zero_atr(), ::testing::ExitedWithCode(2),
              "ATR period must be positive");
}

TEST(ATR, ApplyIndicatorMatchesPush) {
  const CloseSeries close = fixture_close();
  const std::vector<double> high = fixture_high(close);
  const std::vector<double> low = fixture_low(close);
  const std::size_t period = 3U;
  const Column batch = ATR{period}.apply_indicator(high, low, close);

  ATR live{period};
  for (std::size_t i = 0; i < close.samples.size(); ++i) {
    const auto sample =
        live.push(high[i], low[i], close.samples[i]);
    if (!batch.ready[i]) {
      EXPECT_FALSE(sample.has_value());
    } else {
      EXPECT_TRUE(sample.has_value());
      EXPECT_TRUE(near_relative(batch.samples[i], *sample));
    }
  }
}

} // namespace dagobert
