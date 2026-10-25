#include <gtest/gtest.h>

#include "ema.hpp"
#include "rsi.hpp"

#include <cmath>

namespace dagobert {
namespace {

CloseSeries fixture_close_series() {
  CloseSeries close;
  close.samples = {1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0};
  close.ready.assign(close.samples.size(), 1);
  return close;
}

void construct_zero_ema() { EMA{0U}; }
void construct_zero_rsi() { RSI{0U}; }

[[nodiscard]] bool near_relative(double expected, double actual) {
  if (expected == 0.0) {
    return std::abs(actual) < 1e-12;
  }
  return std::abs((actual - expected) / expected) < 1e-12;
}

template <typename Indicator>
void expect_batch_matches_push(const CloseSeries& close, std::size_t period) {
  const Column batch = Indicator{period}.apply_indicator(close);
  Indicator live{period};
  for (std::size_t i = 0; i < close.samples.size(); ++i) {
    const auto sample = live.push(close.samples[i]);
    if (!batch.ready[i]) {
      EXPECT_FALSE(sample.has_value()) << i;
    } else {
      EXPECT_TRUE(sample.has_value()) << i;
      EXPECT_TRUE(near_relative(batch.samples[i], *sample)) << i;
    }
  }
}

} // namespace

TEST(EMA, PeriodZeroExits) {
  EXPECT_EXIT(construct_zero_ema(), ::testing::ExitedWithCode(2),
              "EMA period must be positive");
}

TEST(RSI, PeriodZeroExits) {
  EXPECT_EXIT(construct_zero_rsi(), ::testing::ExitedWithCode(2),
              "RSI period must be positive");
}

TEST(EMA, ApplyIndicatorMatchesPush) {
  expect_batch_matches_push<EMA>(fixture_close_series(), 3U);
}

TEST(RSI, ApplyIndicatorMatchesPush) {
  expect_batch_matches_push<RSI>(fixture_close_series(), 3U);
}

} // namespace dagobert
