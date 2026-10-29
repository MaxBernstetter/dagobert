#include <gtest/gtest.h>

#include "bars_aggregate.hpp"
#include "bars_parquet.hpp"
#include "consensus_strategy.hpp"

#include "google/protobuf/any.pb.h"
#include "tradingcopilot/v1/strategy.pb.h"

#include <filesystem>

namespace dagobert {
namespace {

constexpr std::int64_t kBarDurationMs = 10'800'000;
constexpr std::int64_t kFromMs = 1'704'067'200'000;

std::vector<Bar> fixture_bars() {
  const auto lake =
      std::filesystem::path(DAGOBERT_TEST_FIXTURES_DIR) / "lake_consensus";
  // The lake holds 1h candles; the strategy decides on their 3h aggregates.
  const std::vector<Bar> candles =
      load_bars_from_lake(lake.string(), "demo", "BTC", "USDT", kFromMs,
                          kFromMs + 64 * kBarDurationMs);
  return aggregate_bars(candles, 3'600'000, kBarDurationMs);
}

tradingcopilot::v1::RunEnvelope envelope() {
  tradingcopilot::v1::RunEnvelope out;
  out.set_schema_version(2);
  out.set_label("signal-consensus");
  out.set_initial_quote(1000.0);
  tradingcopilot::v1::SignalConsensusConfig config;
  EXPECT_TRUE(out.mutable_payload()->PackFrom(config));
  return out;
}

} // namespace

TEST(ConsensusStrategy, BacktestMatchesLive) {
  const std::vector<Bar> bars = fixture_bars();
  ASSERT_EQ(bars.size(), 56U);
  const auto env = envelope();

  ConsensusStrategy backtest_path(env, kBarDurationMs);
  const StrategyResult batch = backtest_path.calc_backtest(bars);

  ConsensusStrategy live_path(env, kBarDurationMs);
  StrategyResult streamed;
  for (const Bar &bar : bars) {
    streamed = live_path.calc_live(bar);
  }

  EXPECT_EQ(batch.cash, streamed.cash);
  EXPECT_EQ(batch.qty, streamed.qty);
  EXPECT_EQ(batch.equity, streamed.equity);
  ASSERT_EQ(batch.actions.size(), streamed.actions.size());
  for (std::size_t i = 0; i < batch.actions.size(); ++i) {
    EXPECT_EQ(batch.actions[i].bar_open_ms, streamed.actions[i].bar_open_ms);
    EXPECT_EQ(batch.actions[i].side, streamed.actions[i].side);
    EXPECT_EQ(batch.actions[i].reason, streamed.actions[i].reason);
  }
  ASSERT_EQ(batch.fills.size(), streamed.fills.size());
  for (std::size_t i = 0; i < batch.fills.size(); ++i) {
    EXPECT_EQ(batch.fills[i].ts_ms, streamed.fills[i].ts_ms);
    EXPECT_EQ(batch.fills[i].qty, streamed.fills[i].qty);
    EXPECT_EQ(batch.fills[i].price, streamed.fills[i].price);
  }
  EXPECT_GE(batch.fills.size(), 3U);
}

TEST(ConsensusStrategy, CooldownDefaultsWhenUnset) {
  const auto p = ConsensusStrategy::params_of(envelope());
  EXPECT_EQ(p.min_bars_flat_after_sell, 8U);
  EXPECT_EQ(p.min_bars_long_before_sell, 4U);
}

TEST(ConsensusStrategy, ExplicitZeroCooldownIsHonoured) {
  tradingcopilot::v1::RunEnvelope env = envelope();
  tradingcopilot::v1::SignalConsensusConfig config;
  config.set_min_bars_flat_after_sell(0);
  config.set_min_bars_long_before_sell(0);
  ASSERT_TRUE(env.mutable_payload()->PackFrom(config));
  const auto p = ConsensusStrategy::params_of(env);
  EXPECT_EQ(p.min_bars_flat_after_sell, 0U);
  EXPECT_EQ(p.min_bars_long_before_sell, 0U);
}

TEST(ConsensusStrategy, ExplicitCooldownIsHonoured) {
  tradingcopilot::v1::RunEnvelope env = envelope();
  tradingcopilot::v1::SignalConsensusConfig config;
  config.set_min_bars_flat_after_sell(3);
  ASSERT_TRUE(env.mutable_payload()->PackFrom(config));
  const auto p = ConsensusStrategy::params_of(env);
  EXPECT_EQ(p.min_bars_flat_after_sell, 3U);
  EXPECT_EQ(p.min_bars_long_before_sell, 4U);
}

} // namespace dagobert
