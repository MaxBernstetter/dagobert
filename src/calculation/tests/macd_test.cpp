#include <gtest/gtest.h>

#include "macd.hpp"

#include <cmath>

namespace dagobert {
namespace {

CloseSeries fixture_close_series() {
  CloseSeries close;
  close.samples = {1.0,  2.0,  3.0,  4.0,  5.0,  6.0,  7.0,
                   8.0,  9.0,  10.0, 11.0, 12.0, 13.0, 14.0};
  close.ready.assign(close.samples.size(), 1);
  return close;
}

[[nodiscard]] bool near_relative(double expected, double actual) {
  if (expected == 0.0) {
    return std::abs(actual) < 1e-12;
  }
  return std::abs((actual - expected) / expected) < 1e-12;
}

} // namespace

TEST(MACD, ApplyIndicatorMatchesPush) {
  const CloseSeries close = fixture_close_series();
  const std::size_t fast = 3U;
  const std::size_t slow = 5U;
  const std::size_t signal = 3U;
  const MacdColumns batch = MACD{fast, slow, signal}.apply_indicator(close);

  MACD live{fast, slow, signal};
  for (std::size_t i = 0; i < close.samples.size(); ++i) {
    const MacdSample sample = live.push(close.samples[i]);
    if (!batch.line.ready[i]) {
      EXPECT_FALSE(sample.line.has_value());
    } else {
      EXPECT_TRUE(sample.line.has_value());
      EXPECT_TRUE(near_relative(batch.line.samples[i], *sample.line));
    }
    if (!batch.signal.ready[i]) {
      EXPECT_FALSE(sample.signal.has_value());
    } else {
      EXPECT_TRUE(sample.signal.has_value());
      EXPECT_TRUE(near_relative(batch.signal.samples[i], *sample.signal));
    }
    if (!batch.histogram.ready[i]) {
      EXPECT_FALSE(sample.histogram.has_value());
    } else {
      EXPECT_TRUE(sample.histogram.has_value());
      EXPECT_TRUE(near_relative(batch.histogram.samples[i], *sample.histogram));
    }
  }
}

} // namespace dagobert
