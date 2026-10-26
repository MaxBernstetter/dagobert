#include <gtest/gtest.h>

#include "consensus_signal.hpp"

#include "position.hpp"
#include "quote.hpp"

#include <google/protobuf/struct.pb.h>
#include <google/protobuf/util/json_util.h>

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <fstream>
#include <sstream>
#include <type_traits>

namespace dagobert {
namespace {

struct Fixture {
  CloseSeries close;
  VolumeSeries volume;
  Column signal, regime, macro, rsi, vol_mean, atr, hist;
  std::vector<Bar> bars;

  explicit Fixture(const std::vector<double> &closes) {
    const std::size_t n = closes.size();
    auto fill = [n](Column &c, double v) {
      c.samples.assign(n, v);
      c.ready.assign(n, 1);
    };
    close.samples = closes;
    close.ready.assign(n, 1);
    fill(volume, 2.0);
    fill(signal, 100.0);
    fill(regime, 90.0);
    fill(macro, 80.0);
    fill(rsi, 60.0);
    fill(vol_mean, 1.0);
    fill(atr, 1.0);
    fill(hist, 1.0);
    for (std::size_t i = 0; i < n; ++i) {
      Bar b;
      b.open_ms = static_cast<std::int64_t>(i) * 10'800'000;
      b.open = b.high = b.low = b.close = closes[i];
      b.volume = 2.0;
      b.duration_ms = 10'800'000;
      bars.push_back(b);
    }
  }

  ConsensusColumns columns() const {
    return {close, volume, signal, regime, macro, rsi, vol_mean, atr, hist};
  }
  ConsensusSample sample(std::size_t i) const {
    return {close.samples[i],    volume.samples[i], signal.samples[i],
            regime.samples[i],   macro.samples[i],  rsi.samples[i],
            vol_mean.samples[i], atr.samples[i],    hist.samples[i]};
  }
};

// Cross up at bar 1, down at 3 (too soon after buy), down again at 6.
Fixture crossing() {
  Fixture f({99, 101, 105, 99, 101, 105, 99, 105, 105, 105});
  return f;
}

} // namespace

TEST(ConsensusSignal, ExpectedActionsOnHandBuiltSeries) {
  Fixture f = crossing();
  const ConsensusSeries mask = ConsensusSignal::evaluate(f.columns());
  Position account{Quote::require(1000.0)};
  const auto actions = ConsensusSignal::actions_from(account, f.bars, mask);
  // buy at 1; cross down at 3 is only 2 bars after buy and is skipped;
  // sell at 6; re-entry cross up at 7 is only 1 bar after the sell: skipped.
  ASSERT_EQ(actions.size(), 2U);
  EXPECT_EQ(actions[0].side, Side::buy);
  EXPECT_EQ(actions[0].reason, "consensus_buy");
  EXPECT_EQ(actions[0].bar_open_ms, 1 * 10'800'000);
  EXPECT_EQ(actions[1].side, Side::sell);
  EXPECT_EQ(actions[1].reason, "consensus_sell");
  EXPECT_EQ(actions[1].bar_open_ms, 6 * 10'800'000);
}

TEST(ConsensusSignal, BearVolumeVoteBlocksEntry) {
  Fixture f = crossing();
  f.volume.samples[1] = 0.5;
  const ConsensusSeries mask = ConsensusSignal::evaluate(f.columns());
  Position account{Quote::require(1000.0)};
  const auto actions = ConsensusSignal::actions_from(account, f.bars, mask);
  ASSERT_FALSE(actions.empty());
  EXPECT_NE(actions.front().bar_open_ms, 1 * 10'800'000);
}

TEST(ConsensusSignal, AtrStopSells) {
  Fixture f({99, 101, 105, 99, 99, 99, 90});
  const ConsensusSeries mask = ConsensusSignal::evaluate(f.columns());
  Position account{Quote::require(1000.0)};
  const auto actions = ConsensusSignal::actions_from(account, f.bars, mask);
  ASSERT_EQ(actions.size(), 2U);
  EXPECT_EQ(actions[1].reason, "atr_stop");
}

TEST(ConsensusSignal, SeriesMatchesSample) {
  Fixture f = crossing();
  const ConsensusSeries mask = ConsensusSignal::evaluate(f.columns());
  Position batch{Quote::require(1000.0)};
  const auto expected = ConsensusSignal::actions_from(batch, f.bars, mask);

  ConsensusSignal live;
  Position account{Quote::require(1000.0)};
  std::vector<Action> got;
  for (std::size_t i = 0; i < f.bars.size(); ++i) {
    const auto rule = live.apply(f.sample(i), account.flat());
    if (rule) {
      live.execute(account, f.bars[i], *rule);
      got.push_back(account.actions().back());
    }
  }
  ASSERT_EQ(got.size(), expected.size());
  for (std::size_t i = 0; i < got.size(); ++i) {
    EXPECT_EQ(got[i].bar_open_ms, expected[i].bar_open_ms);
    EXPECT_EQ(got[i].side, expected[i].side);
    EXPECT_EQ(got[i].reason, expected[i].reason);
  }
}

namespace {

using google::protobuf::Struct;

// Replays tests/fixtures/consensus_sizing.json, generated from the sized Go
// signal-consensus playbook (see calculation-signal/.../cpp_fixture_test.go).
struct GoFixture {
  Fixture f;
  sizing::SizingConfig cfg;
  double initial_quote{};
  std::vector<Struct> expected;

  static Struct load() {
    std::ifstream in(CONSENSUS_FIXTURE_PATH);
    std::stringstream text;
    text << in.rdbuf();
    Struct root;
    const auto status =
        google::protobuf::util::JsonStringToMessage(text.str(), &root);
    EXPECT_TRUE(status.ok()) << status.message();
    return root;
  }

  static Fixture build(const Struct &root) {
    const auto &bars = root.fields().at("bars").list_value().values();
    std::vector<double> closes;
    for (const auto &v : bars) {
      closes.push_back(v.struct_value().fields().at("close").number_value());
    }
    Fixture f(closes);
    for (int i = 0; i < bars.size(); ++i) {
      const auto &b = bars[i].struct_value().fields();
      auto num = [&b](const char *k) { return b.at(k).number_value(); };
      const auto u = static_cast<std::size_t>(i);
      f.volume.samples[u] = f.bars[u].volume = num("volume");
      f.signal.samples[u] = num("signal");
      f.regime.samples[u] = num("regime");
      f.macro.samples[u] = num("macro");
      f.rsi.samples[u] = num("rsi");
      f.hist.samples[u] = num("hist");
      f.atr.samples[u] = num("atr");
      f.vol_mean.samples[u] = num("vol_mean");
    }
    return f;
  }

  explicit GoFixture(const Struct &root) : f(build(root)) {
    initial_quote = root.fields().at("initial_quote").number_value();
    const auto &sz =
        root.fields().at("config").struct_value().fields().at("sizing");
    const auto &m = sz.struct_value().fields();
    cfg.unit_risk_pct = m.at("unitRiskPct").number_value();
    cfg.heat_pct = m.at("heatPct").number_value();
    cfg.max_units = static_cast<int>(m.at("maxUnits").number_value());
    cfg.add_atr_mult = m.at("addAtrMult").number_value();
    for (const auto &e : root.fields().at("expected").list_value().values()) {
      expected.push_back(e.struct_value());
    }
  }
};

void expect_matches_go(const GoFixture &g, const Position &account,
                       const std::vector<Action> &actions) {
  ASSERT_EQ(actions.size(), g.expected.size());
  ASSERT_EQ(account.fills().size(), g.expected.size());
  for (std::size_t i = 0; i < actions.size(); ++i) {
    const auto &e = g.expected[i].fields();
    EXPECT_EQ(actions[i].bar_open_ms,
              static_cast<std::int64_t>(e.at("bar_open_ms").number_value()));
    EXPECT_EQ(actions[i].reason, e.at("reason").string_value());
    EXPECT_EQ(account.fills()[i].side, e.at("side").string_value());
    EXPECT_NEAR(account.fills()[i].price, e.at("price").number_value(), 1e-9);
    EXPECT_NEAR(account.fills()[i].qty, e.at("qty").number_value(), 1e-6);
  }
}

} // namespace

TEST(ConsensusSizing, MatchesGoSizedPlaybook) {
  const GoFixture g{GoFixture::load()};
  const ConsensusSeries mask = ConsensusSignal::evaluate(g.f.columns());
  Position account{Quote::require(g.initial_quote)};
  const auto actions =
      ConsensusSignal::actions_from(account, g.f.bars, mask, g.cfg);
  expect_matches_go(g, account, actions);
}

TEST(ConsensusSizing, StreamingMatchesGoSizedPlaybook) {
  const GoFixture g{GoFixture::load()};
  ConsensusSignal live{g.cfg};
  Position account{Quote::require(g.initial_quote)};
  std::vector<Action> got;
  for (std::size_t i = 0; i < g.f.bars.size(); ++i) {
    if (const auto rule = live.apply(g.f.sample(i), account)) {
      live.execute(account, g.f.bars[i], *rule);
      got.push_back(account.actions().back());
    }
  }
  expect_matches_go(g, account, got);
}

TEST(ConsensusSizing, UsesEveryRuleAndSizesFromTheStop) {
  const GoFixture g{GoFixture::load()};
  const ConsensusSeries mask = ConsensusSignal::evaluate(g.f.columns());
  Position account{Quote::require(g.initial_quote)};
  const auto actions =
      ConsensusSignal::actions_from(account, g.f.bars, mask, g.cfg);
  std::vector<std::string> reasons;
  for (const Action &a : actions) {
    reasons.push_back(a.reason);
  }
  for (const char *want :
       {"consensus_buy", "pyramid", "vol_cut", "atr_stop", "consensus_sell"}) {
    EXPECT_NE(std::ranges::find(reasons, want), reasons.end()) << want;
  }
  // Entry: 0.5% of 100000 over a 2.5 * 2 ATR stop at 101.
  EXPECT_NEAR(account.fills().front().qty * 101.0, 10100.0, 1e-6);
  EXPECT_NEAR(actions.front().stop, 96.0, 1e-9);
}

TEST(ConsensusSizing, BuyBelowMinOrderIsSkipped) {
  Fixture f = crossing();
  sizing::SizingConfig cfg;
  cfg.min_order_quote = 1e12;
  const ConsensusSeries mask = ConsensusSignal::evaluate(f.columns());
  Position account{Quote::require(1000.0)};
  EXPECT_TRUE(
      ConsensusSignal::actions_from(account, f.bars, mask, cfg).empty());
  cfg.min_order_quote = 0.0;
  Position again{Quote::require(1000.0)};
  EXPECT_FALSE(ConsensusSignal::actions_from(again, f.bars, mask, cfg).empty());
}

TEST(ConsensusSizing, NonPositiveStopStillBuysLikeGo) {
  Fixture f = crossing();
  f.atr.samples.assign(f.atr.samples.size(), 100.0); // stop = 101 - 250 < 0
  sizing::SizingConfig cfg;
  const ConsensusSeries mask = ConsensusSignal::evaluate(f.columns());
  Position account{Quote::require(100000.0)};
  const auto actions =
      ConsensusSignal::actions_from(account, f.bars, mask, cfg);
  ASSERT_FALSE(actions.empty());
  EXPECT_EQ(actions.front().reason, "consensus_buy");
  EXPECT_LT(actions.front().stop, 0.0);
}

TEST(ConsensusSizing, WithoutSizingKeepsWholePositionTrades) {
  const GoFixture g{GoFixture::load()};
  const ConsensusSeries mask = ConsensusSignal::evaluate(g.f.columns());
  Position account{Quote::require(g.initial_quote)};
  const auto actions = ConsensusSignal::actions_from(account, g.f.bars, mask);
  ASSERT_FALSE(actions.empty());
  EXPECT_EQ(actions.front().reason, "consensus_buy");
  EXPECT_NEAR(actions.front().weight_pct, 100.0, 1e-12);
  for (const Action &a : actions) {
    EXPECT_NE(a.reason, "pyramid");
    EXPECT_NE(a.reason, "vol_cut");
  }
}

TEST(ConsensusSizing, ResetClearsUnitsAndKeepsConfig) {
  const GoFixture g{GoFixture::load()};
  ConsensusSignal live{g.cfg};
  Position account{Quote::require(g.initial_quote)};
  for (std::size_t i = 0; i < 4; ++i) {
    if (const auto rule = live.apply(g.f.sample(i), account)) {
      live.execute(account, g.f.bars[i], *rule);
    }
  }
  ASSERT_FALSE(account.flat());
  live.reset();
  Position fresh{Quote::require(g.initial_quote)};
  std::vector<Action> got;
  for (std::size_t i = 0; i < g.f.bars.size(); ++i) {
    if (const auto rule = live.apply(g.f.sample(i), fresh)) {
      live.execute(fresh, g.f.bars[i], *rule);
      got.push_back(fresh.actions().back());
    }
  }
  expect_matches_go(g, fresh, got);
}

namespace {

std::vector<Action> run_with(const Fixture &f, const ConsensusParams &p) {
  const ConsensusSeries mask = ConsensusSignal::evaluate(f.columns(), p);
  Position account{Quote::require(1000.0)};
  return ConsensusSignal::actions_from(account, f.bars, mask, std::nullopt, p);
}

ConsensusParams params_from(const Struct &config) {
  ConsensusParams p;
  const auto &m = config.fields();
  const auto num = [&m](const char *k, auto &out) {
    if (const auto it = m.find(k); it != m.end()) {
      out = static_cast<std::remove_reference_t<decltype(out)>>(
          it->second.number_value());
    }
  };
  num("rsiBull", p.rsi_bull);
  num("rsiBear", p.rsi_bear);
  num("volRatioBull", p.vol_ratio_bull);
  num("volRatioBear", p.vol_ratio_bear);
  num("atrStopMult", p.atr_stop_mult);
  num("minBarsFlatAfterSell", p.min_bars_flat_after_sell);
  num("minBarsLongBeforeSell", p.min_bars_long_before_sell);
  return p;
}

Struct load_params_fixture() {
  std::ifstream in(CONSENSUS_PARAMS_FIXTURE_PATH);
  std::stringstream text;
  text << in.rdbuf();
  Struct root;
  const auto status =
      google::protobuf::util::JsonStringToMessage(text.str(), &root);
  EXPECT_TRUE(status.ok()) << status.message();
  return root;
}

} // namespace

TEST(ConsensusParams, DefaultsEqualGoPlaybook) {
  const ConsensusParams p;
  EXPECT_EQ(p.rsi_bull, 52.0);
  EXPECT_EQ(p.rsi_bear, 48.0);
  EXPECT_EQ(p.vol_ratio_bull, 1.10);
  EXPECT_EQ(p.vol_ratio_bear, 0.85);
  EXPECT_EQ(p.atr_stop_mult, 2.5);
  EXPECT_EQ(p.min_bars_flat_after_sell, 8U);
  EXPECT_EQ(p.min_bars_long_before_sell, 4U);
  EXPECT_EQ(run_with(crossing(), p).size(), 2U);
}

TEST(ConsensusParams, RsiBullGatesEntryWithoutHistogram) {
  Fixture f = crossing();
  f.hist.ready.assign(f.hist.ready.size(), 0); // MACD falls back to momentum
  EXPECT_FALSE(run_with(f, {}).empty());
  ConsensusParams p;
  p.rsi_bull = 70.0;
  EXPECT_TRUE(run_with(f, p).empty());
}

TEST(ConsensusParams, RsiBearBlocksEntry) {
  Fixture f = crossing();
  f.rsi.samples.assign(f.rsi.samples.size(), 40.0);
  EXPECT_TRUE(run_with(f, {}).empty());
  ConsensusParams p;
  p.rsi_bear = 30.0;
  EXPECT_FALSE(run_with(f, p).empty());
}

TEST(ConsensusParams, VolRatioBearBlocksEntry) {
  Fixture f = crossing();
  f.volume.samples[1] = 0.5;
  EXPECT_NE(run_with(f, {}).front().bar_open_ms, 1 * 10'800'000);
  ConsensusParams p;
  p.vol_ratio_bear = 0.4;
  EXPECT_EQ(run_with(f, p).front().bar_open_ms, 1 * 10'800'000);
}

TEST(ConsensusParams, AtrStopMultMovesTheStop) {
  Fixture f({99, 101, 105, 99, 99, 99, 90});
  EXPECT_EQ(run_with(f, {}).size(), 2U);
  ConsensusParams p;
  p.atr_stop_mult = 20.0;
  EXPECT_EQ(run_with(f, p).size(), 1U);
}

TEST(ConsensusParams, MinBarsFlatAfterSellReenters) {
  ConsensusParams p;
  p.min_bars_flat_after_sell = 1; // crossing(): sell at 6, cross up at 7
  const auto actions = run_with(crossing(), p);
  ASSERT_GE(actions.size(), 3U);
  EXPECT_EQ(actions[2].bar_open_ms, 7 * 10'800'000);
}

TEST(ConsensusParams, MinBarsLongBeforeSellExitsEarlier) {
  ConsensusParams p;
  p.min_bars_long_before_sell = 2; // crossing(): buy at 1, cross down at 3
  const auto actions = run_with(crossing(), p);
  ASSERT_GE(actions.size(), 2U);
  EXPECT_EQ(actions[1].bar_open_ms, 3 * 10'800'000);
}

TEST(ConsensusParams, SizedStopFollowsAtrStopMult) {
  const Fixture f = crossing();
  ConsensusParams p;
  p.atr_stop_mult = 4.0;
  const ConsensusSeries mask = ConsensusSignal::evaluate(f.columns(), p);
  Position account{Quote::require(100000.0)};
  const auto actions = ConsensusSignal::actions_from(account, f.bars, mask,
                                                     sizing::SizingConfig{}, p);
  ASSERT_FALSE(actions.empty());
  EXPECT_NEAR(actions.front().stop, 101.0 - 4.0, 1e-9); // close 101, ATR 1
}

TEST(ConsensusParams, MatchesGoPlaybookForDefaultAndNonDefaultConfig) {
  const Struct root = load_params_fixture();
  const Fixture f = GoFixture::build(root);
  const double quote = root.fields().at("initial_quote").number_value();
  const auto &cases = root.fields().at("cases").list_value().values();
  ASSERT_EQ(cases.size(), 2);
  for (const auto &c : cases) {
    const auto &fields = c.struct_value().fields();
    const ConsensusParams p = params_from(fields.at("config").struct_value());
    const auto &expected = fields.at("expected").list_value().values();
    const std::string name = fields.at("name").string_value();
    const ConsensusSeries mask = ConsensusSignal::evaluate(f.columns(), p);
    Position batch{Quote::require(quote)};
    const auto actions =
        ConsensusSignal::actions_from(batch, f.bars, mask, std::nullopt, p);

    ConsensusSignal live{std::nullopt, p};
    Position streamed{Quote::require(quote)};
    std::vector<Action> got;
    for (std::size_t i = 0; i < f.bars.size(); ++i) {
      if (const auto rule = live.apply(f.sample(i), streamed.flat())) {
        live.execute(streamed, f.bars[i], *rule);
        got.push_back(streamed.actions().back());
      }
    }
    ASSERT_EQ(actions.size(), expected.size()) << name;
    ASSERT_EQ(got.size(), expected.size()) << name;
    for (std::size_t i = 0; i < actions.size(); ++i) {
      const auto &e = expected[i].struct_value().fields();
      const auto at =
          static_cast<std::int64_t>(e.at("bar_open_ms").number_value());
      EXPECT_EQ(actions[i].bar_open_ms, at) << name;
      EXPECT_EQ(got[i].bar_open_ms, at) << name;
      EXPECT_EQ(actions[i].reason, e.at("reason").string_value()) << name;
      EXPECT_EQ(batch.fills()[i].side, e.at("side").string_value()) << name;
      EXPECT_NEAR(batch.fills()[i].qty, e.at("qty").number_value(), 1e-6)
          << name;
    }
  }
}

} // namespace dagobert
