// Go-vs-C++ cross-check: the recorded fixture holds bars plus the actions and
// fills the Go sized ma-cross playbook produced for them (regenerate with
// src/calculation-signal/playbooks/ma-cross/cppfixture_test.go). The C++
// strategy must reproduce them, in backtest and bar by bar in live.
#include "tradingcopilot/v1/run_envelope.pb.h"
#include <gtest/gtest.h>

#include "sma_cross_strategy.hpp"

#include "google/protobuf/any.pb.h"
#include "tradingcopilot/v1/strategy.pb.h"

#include <google/protobuf/struct.pb.h>
#include <google/protobuf/util/json_util.h>
#include <magic_enum/magic_enum.hpp>

#include <cmath>
#include <fstream>
#include <set>
#include <sstream>

namespace dagobert {
namespace {

using google::protobuf::Struct;

constexpr double kTol = 1e-9;

Struct load_fixture() {
  std::ifstream in(SMA_CROSS_SIZING_FIXTURE_PATH);
  std::stringstream text;
  text << in.rdbuf();
  Struct root;
  const auto status =
      google::protobuf::util::JsonStringToMessage(text.str(), &root);
  EXPECT_TRUE(status.ok()) << status.message();
  return root;
}

double num(const Struct &s, const char *key) {
  return s.fields().at(key).number_value();
}

void expect_near(double want, double got, const char *what, std::size_t i) {
  EXPECT_NEAR(want, got, kTol * std::max(1.0, std::abs(want)))
      << what << " #" << i;
}

std::vector<Bar> bars_of(const Struct &root) {
  std::vector<Bar> bars;
  for (const auto &v : root.fields().at("bars").list_value().values()) {
    const Struct &b = v.struct_value();
    Bar bar;
    bar.open_ms = static_cast<std::int64_t>(num(b, "openMs"));
    bar.high = num(b, "high");
    bar.low = num(b, "low");
    bar.close = num(b, "close");
    bar.open = bar.close;
    bar.volume = 1.0;
    bar.duration_ms = static_cast<std::int64_t>(num(root, "barMs"));
    bars.push_back(bar);
  }
  return bars;
}

sizing::SizingConfig sizing_of(const Struct &root) {
  const Struct &s = root.fields().at("sizing").struct_value();
  sizing::SizingConfig cfg;
  cfg.unit_risk_pct = num(s, "unitRiskPct");
  cfg.heat_pct = num(s, "heatPct");
  cfg.max_units = static_cast<int>(num(s, "maxUnits"));
  cfg.stop_atr_mult = num(s, "stopAtrMult");
  cfg.add_atr_mult = num(s, "addAtrMult");
  if (s.fields().contains("minOrderQuote")) {
    cfg.min_order_quote = num(s, "minOrderQuote");
  }
  return cfg;
}

tradingcopilot::v1::RunEnvelope envelope_of(const Struct &root) {
  tradingcopilot::v1::RunEnvelope envelope;
  envelope.set_schema_version(2);
  envelope.set_label("sma-cross");
  envelope.set_initial_quote(num(root, "initialQuote"));
  tradingcopilot::v1::MaCrossConfig config;
  config.set_fast_hr(static_cast<std::int32_t>(num(root, "fastHr")));
  config.set_slow_hr(static_cast<std::int32_t>(num(root, "slowHr")));
  google::protobuf::Any payload;
  EXPECT_TRUE(payload.PackFrom(config));
  payload.set_type_url("type.googleapis.com/tradingcopilot.v1.MaCrossConfig");
  *envelope.mutable_payload() = payload;
  return envelope;
}

void expect_matches_go(const Struct &root, const StrategyResult &got) {
  const auto &actions = root.fields().at("actions").list_value().values();
  const auto &fills = root.fields().at("fills").list_value().values();
  ASSERT_EQ(static_cast<std::size_t>(actions.size()), got.actions.size());
  ASSERT_EQ(static_cast<std::size_t>(fills.size()), got.fills.size());
  for (std::size_t i = 0; i < got.actions.size(); ++i) {
    const Struct &want = actions[static_cast<int>(i)].struct_value();
    EXPECT_EQ(static_cast<std::int64_t>(num(want, "barOpenMs")),
              got.actions[i].bar_open_ms)
        << i;
    EXPECT_EQ(want.fields().at("side").string_value(),
              magic_enum::enum_name(got.actions[i].side))
        << i;
    EXPECT_EQ(want.fields().at("reason").string_value(), got.actions[i].reason)
        << i;
    if (want.fields().contains("stop")) {
      expect_near(num(want, "stop"), got.actions[i].stop, "stop", i);
    }
  }
  for (std::size_t i = 0; i < got.fills.size(); ++i) {
    const Struct &want = fills[static_cast<int>(i)].struct_value();
    EXPECT_EQ(want.fields().at("side").string_value(), got.fills[i].side) << i;
    expect_near(num(want, "qty"), got.fills[i].qty, "qty", i);
    expect_near(num(want, "price"), got.fills[i].price, "price", i);
  }
  expect_near(num(root, "finalCash"), got.cash, "cash", 0);
  expect_near(num(root, "finalQty"), got.qty, "qty", 0);
}

} // namespace

TEST(SmaCrossSizing, BacktestMatchesGo) {
  const Struct root = load_fixture();
  SmaCrossStrategy strategy{envelope_of(root),
                            static_cast<std::int64_t>(num(root, "barMs")),
                            sizing_of(root)};
  const std::vector<Bar> bars = bars_of(root);
  expect_matches_go(root, strategy.calc_backtest(bars));
}

TEST(SmaCrossSizing, LiveMatchesGoAndBacktest) {
  const Struct root = load_fixture();
  SmaCrossStrategy strategy{envelope_of(root),
                            static_cast<std::int64_t>(num(root, "barMs")),
                            sizing_of(root)};
  StrategyResult last;
  for (const Bar &bar : bars_of(root)) {
    last = strategy.calc_live(bar);
  }
  expect_matches_go(root, last);

  // reset() clears indicators, book and open units: a second run repeats.
  strategy.reset();
  for (const Bar &bar : bars_of(root)) {
    last = strategy.calc_live(bar);
  }
  expect_matches_go(root, last);
}

TEST(SmaCrossSizing, OrdersBelowMinOrderAreSkipped) {
  const Struct root = load_fixture();
  sizing::SizingConfig cfg = sizing_of(root);
  cfg.min_order_quote = 1e12;
  SmaCrossStrategy strategy{envelope_of(root),
                            static_cast<std::int64_t>(num(root, "barMs")), cfg};
  const StrategyResult got = strategy.calc_backtest(bars_of(root));
  EXPECT_TRUE(got.actions.empty());
  EXPECT_TRUE(got.fills.empty());
}

TEST(SmaCrossSizing, FixtureCoversEveryRule) {
  const Struct root = load_fixture();
  std::set<std::string> rules;
  for (const auto &v : root.fields().at("actions").list_value().values()) {
    rules.insert(v.struct_value().fields().at("reason").string_value());
  }
  for (const char *rule :
       {"golden_cross", "atr_stop", "death_cross", "vol_cut", "pyramid"}) {
    EXPECT_TRUE(rules.contains(rule)) << rule;
  }
}

} // namespace dagobert
