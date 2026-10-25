#include <gtest/gtest.h>

#include "bars_parquet.hpp"

#include <cmath>
#include <filesystem>

namespace dagobert {
namespace {

constexpr std::int64_t kOpenMs = 1'704'067'200'000; // 2024-01-01 00:00:00 UTC
constexpr std::int64_t kIntervalMs = 10'800'000;

} // namespace

TEST(ParquetBars, FixtureRoundTrip) {
  const std::filesystem::path root =
      std::filesystem::path(__FILE__).parent_path() / "fixtures" / "lake";
  const std::vector<Bar> bars = load_bars_from_lake(
      root.string(), "demo", "BTC", "USDT", kOpenMs, kOpenMs + kIntervalMs);
  ASSERT_EQ(bars.size(), 1U);
  EXPECT_EQ(bars[0].open_ms, kOpenMs);
  EXPECT_NEAR(bars[0].open, 100.25, 1e-9);
  EXPECT_NEAR(bars[0].high, 101.50, 1e-9);
  EXPECT_NEAR(bars[0].low, 99.75, 1e-9);
  EXPECT_NEAR(bars[0].close, 100.50, 1e-9);
  EXPECT_NEAR(bars[0].volume, 12.345, 1e-9);
  EXPECT_EQ(bars[0].duration_ms, kIntervalMs);
}

} // namespace dagobert
