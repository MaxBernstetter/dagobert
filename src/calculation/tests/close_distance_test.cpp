#include <gtest/gtest.h>

#include "close_distance.hpp"

#include <cmath>

namespace dagobert {
namespace {

CloseSeries fixture_close_series() {
  CloseSeries close;
  close.samples = {1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0};
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

TEST(CloseDistance, ApplyIndicatorMatchesPush) {
  const CloseSeries close = fixture_close_series();
  const std::size_t period = 3U;
  const Column batch = CloseDistance{period}.apply_indicator(close);

  CloseDistance live{period};
  for (std::size_t i = 0; i < close.samples.size(); ++i) {
    const auto sample = live.push(close.samples[i]);
    if (!batch.ready[i]) {
      EXPECT_FALSE(sample.has_value());
    } else {
      EXPECT_TRUE(sample.has_value());
      EXPECT_TRUE(near_relative(batch.samples[i], *sample));
    }
  }
}

} // namespace dagobert
