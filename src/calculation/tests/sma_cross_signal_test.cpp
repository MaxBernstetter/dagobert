#include <gtest/gtest.h>

#include "sma.hpp"
#include "sma_cross_signal.hpp"

#include "position.hpp"
#include "quote.hpp"

namespace dagobert {
namespace {

CloseSeries make_close(std::initializer_list<double> values) {
  CloseSeries close;
  close.samples.assign(values);
  close.ready.assign(close.samples.size(), 1);
  return close;
}

std::vector<Bar> bars_for_closes(const CloseSeries &close) {
  std::vector<Bar> bars;
  bars.reserve(close.samples.size());
  std::int64_t open_ms = 0;
  for (const double price : close.samples) {
    Bar bar;
    bar.open_ms = open_ms;
    bar.close = price;
    bar.high = price;
    bar.low = price;
    bar.open = price;
    bar.volume = 1.0;
    bar.duration_ms = 10'800'000;
    bars.push_back(bar);
    open_ms += bar.duration_ms;
  }
  return bars;
}

} // namespace

TEST(SmaCrossSignal, EvaluateMatchesApply) {
  const CloseSeries close =
      make_close({1.0, 2.0, 3.0, 2.0, 1.0, 2.0, 3.0, 4.0});
  const CloseSeries fast = SMA{2U}.apply_indicator(close);
  const CloseSeries slow = SMA{4U}.apply_indicator(close);
  const SmaCrossSeries batch = SmaCrossSignal::evaluate(fast, slow);

  SmaCrossSignal live;
  SMA fast_live{2U};
  SMA slow_live{4U};
  bool flat = true;
  for (std::size_t i = 0; i < close.samples.size(); ++i) {
    const auto rule = live.apply(fast_live.push(close.samples[i]),
                                 slow_live.push(close.samples[i]), flat);
    if (!batch.rule[i].has_value()) {
      EXPECT_FALSE(rule.has_value()) << i;
    } else if (*batch.rule[i] == SmaCrossRule::death_cross && flat) {
      EXPECT_FALSE(rule.has_value()) << i;
    } else {
      EXPECT_TRUE(rule.has_value()) << i;
      EXPECT_EQ(*batch.rule[i], *rule);
    }
    if (rule == SmaCrossRule::golden_cross) {
      flat = false;
    } else if (rule == SmaCrossRule::death_cross) {
      flat = true;
    }
  }
}

TEST(SmaCrossSignal, ActionsFromRoundTrip) {
  const CloseSeries close = make_close(
      {10.0, 10.0, 10.0, 10.0, 20.0, 20.0, 20.0, 20.0, 10.0, 10.0, 10.0, 10.0});
  const CloseSeries fast = SMA{2U}.apply_indicator(close);
  const CloseSeries slow = SMA{4U}.apply_indicator(close);
  const SmaCrossSeries mask = SmaCrossSignal::evaluate(fast, slow);
  const std::vector<Bar> bars = bars_for_closes(close);

  Position account{Quote::require(1000.0)};
  const std::vector<Action> actions =
      SmaCrossSignal::actions_from(account, bars, mask);
  ASSERT_GE(actions.size(), 2U);
  EXPECT_EQ(actions.front().reason, "golden_cross");
  EXPECT_EQ(actions.back().reason, "death_cross");
  EXPECT_EQ(account.fills().size(), 2U);
}

TEST(SmaCrossSignal, LeadingDeathCrossWhileFlatStaysFlat) {
  const CloseSeries close = make_close({10.0, 9.0, 8.0, 7.0, 6.0, 5.0});
  const CloseSeries fast = SMA{1U}.apply_indicator(close);
  const CloseSeries slow = SMA{2U}.apply_indicator(close);
  const SmaCrossSeries mask = SmaCrossSignal::evaluate(fast, slow);
  const std::vector<Bar> bars = bars_for_closes(close);

  Position account{Quote::require(1000.0)};
  (void)SmaCrossSignal::actions_from(account, bars, mask);
  EXPECT_TRUE(account.flat());
  EXPECT_TRUE(account.actions().empty());
}

} // namespace dagobert
