#include <gtest/gtest.h>

#include "sma.hpp"

#include <cmath>
#include <vector>

namespace dagobert {
namespace {

CloseSeries fixture_close_series() {
  CloseSeries close;
  close.samples = {1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0};
  close.ready.assign(close.samples.size(), 1);
  return close;
}

void construct_zero_period() { SMA{0U}; }

[[nodiscard]] bool near_relative(double expected, double actual) {
  if (expected == 0.0) {
    return std::abs(actual) < 1e-12;
  }
  return std::abs((actual - expected) / expected) < 1e-12;
}

} // namespace

TEST(SMA, PeriodZeroExits) {
  EXPECT_EXIT(construct_zero_period(), ::testing::ExitedWithCode(2),
              "SMA period must be positive");
}

TEST(SMA, ApplyIndicatorMatchesPush) {
  const CloseSeries close = fixture_close_series();
  const std::size_t period = 3U;
  const CloseSeries batch = SMA{period}.apply_indicator(close);

  SMA live{period};
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

TEST(SMA, VolumeOverloadMatchesPush) {
  VolumeSeries volume;
  volume.samples = {10.0, 20.0, 30.0, 40.0, 50.0};
  volume.ready.assign(volume.samples.size(), 1);

  const std::size_t period = 2U;
  const VolumeSeries batch = SMA{period}.apply_indicator(volume);
  SMA live{period};
  for (std::size_t i = 0; i < volume.samples.size(); ++i) {
    const auto sample = live.push(volume.samples[i]);
    if (!batch.ready[i]) {
      EXPECT_FALSE(sample.has_value());
    } else {
      EXPECT_TRUE(sample.has_value());
      EXPECT_TRUE(near_relative(batch.samples[i], *sample));
    }
  }
}

} // namespace dagobert
