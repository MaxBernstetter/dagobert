#include <gtest/gtest.h>

#include "sma_cross_strategy.hpp"

#include "google/protobuf/any.pb.h"
#include "tradingcopilot/v1/strategy.pb.h"

namespace dagobert {
namespace {

constexpr std::int64_t kBarDurationMs = 10'800'000;

std::vector<Bar> fixture_bars() {
  const double prices[] = {10.0, 10.0, 10.0, 10.0, 20.0, 20.0,
                           20.0, 20.0, 10.0, 10.0, 10.0, 10.0};
  std::vector<Bar> bars;
  std::int64_t open_ms = 0;
  for (const double close : prices) {
    Bar bar;
    bar.open_ms = open_ms;
    bar.open = close;
    bar.high = close;
    bar.low = close;
    bar.close = close;
    bar.volume = 1.0;
    bar.duration_ms = kBarDurationMs;
    bars.push_back(bar);
    open_ms += kBarDurationMs;
  }
  return bars;
}

void fill_ma_cross_envelope(tradingcopilot::v1::RunEnvelope &envelope) {
  envelope.set_schema_version(2);
  envelope.set_label("sma-cross");
  envelope.set_initial_quote(1000.0);

  tradingcopilot::v1::MaCrossConfig config;
  config.set_fast_hr(3);
  config.set_slow_hr(6);

  google::protobuf::Any payload;
  ASSERT_TRUE(payload.PackFrom(config));
  payload.set_type_url("type.googleapis.com/tradingcopilot.v1.MaCrossConfig");
  *envelope.mutable_payload() = payload;
}

bool results_match(const StrategyResult &backtest, const StrategyResult &live) {
  if (backtest.cash != live.cash || backtest.qty != live.qty ||
      backtest.equity != live.equity ||
      backtest.actions.size() != live.actions.size() ||
      backtest.fills.size() != live.fills.size()) {
    return false;
  }
  for (std::size_t i = 0; i < backtest.actions.size(); ++i) {
    if (backtest.actions[i].bar_open_ms != live.actions[i].bar_open_ms ||
        backtest.actions[i].side != live.actions[i].side ||
        backtest.actions[i].reason != live.actions[i].reason ||
        backtest.actions[i].weight_pct != live.actions[i].weight_pct) {
      return false;
    }
  }
  for (std::size_t i = 0; i < backtest.fills.size(); ++i) {
    if (backtest.fills[i].ts_ms != live.fills[i].ts_ms ||
        backtest.fills[i].side != live.fills[i].side ||
        backtest.fills[i].qty != live.fills[i].qty ||
        backtest.fills[i].price != live.fills[i].price ||
        backtest.fills[i].reason != live.fills[i].reason) {
      return false;
    }
  }
  return true;
}

} // namespace

TEST(SmaCrossStrategy, BacktestMatchesLive) {
  const std::vector<Bar> bars = fixture_bars();
  tradingcopilot::v1::RunEnvelope envelope;
  fill_ma_cross_envelope(envelope);

  SmaCrossStrategy backtest_path(envelope, kBarDurationMs);
  const StrategyResult batch = backtest_path.calc_backtest(bars);

  SmaCrossStrategy live_path(envelope, kBarDurationMs);
  StrategyResult streamed;
  for (const Bar &bar : bars) {
    streamed = live_path.calc_live(bar);
  }

  EXPECT_TRUE(results_match(batch, streamed));
  EXPECT_GE(batch.fills.size(), 2U);
}

} // namespace dagobert
