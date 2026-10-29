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
      std::filesystem::path(DAGOBERT_TEST_FIXTURES_DIR) / "lake";
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

// The decimal schema (lake) and the old string schema (lake_legacy) hold the
// same logical bars; both must load to bit-identical doubles, including a
// volume that does not fit int64 at scale 8.
TEST(ParquetBars, DecimalAndLegacySchemasAgree) {
  const std::filesystem::path fixtures =
      std::filesystem::path(DAGOBERT_TEST_FIXTURES_DIR);
  const std::vector<Bar> dec =
      load_bars_from_lake((fixtures / "lake").string(), "demo", "BTC", "USDT",
                          kOpenMs, kOpenMs + 2 * kIntervalMs);
  const std::vector<Bar> old =
      load_bars_from_lake((fixtures / "lake_legacy").string(), "demo", "BTC",
                          "USDT", kOpenMs, kOpenMs + 2 * kIntervalMs);
  ASSERT_EQ(dec.size(), 2U);
  ASSERT_EQ(old.size(), 2U);
  for (std::size_t i = 0; i < 2; ++i) {
    EXPECT_EQ(dec[i].open_ms, old[i].open_ms);
    EXPECT_EQ(dec[i].open, old[i].open);
    EXPECT_EQ(dec[i].high, old[i].high);
    EXPECT_EQ(dec[i].low, old[i].low);
    EXPECT_EQ(dec[i].close, old[i].close);
    EXPECT_EQ(dec[i].volume, old[i].volume);
    EXPECT_EQ(dec[i].duration_ms, old[i].duration_ms);
  }
  EXPECT_EQ(dec[1].low, 1e-8);
  EXPECT_EQ(dec[1].volume, 250000000000.12345678);
}

} // namespace dagobert
