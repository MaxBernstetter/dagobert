#include <gtest/gtest.h>

#include "consensus_signal.hpp"

#include "position.hpp"
#include "quote.hpp"

#include <cstdint>

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

} // namespace dagobert
