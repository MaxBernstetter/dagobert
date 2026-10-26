#include <gtest/gtest.h>

#include "consensus_gates.hpp"

namespace dagobert {
namespace {

Column col(std::vector<double> v, std::size_t warmup = 0) {
  Column c;
  c.samples = std::move(v);
  c.ready.assign(c.samples.size(), 1);
  for (std::size_t i = 0; i < warmup && i < c.ready.size(); ++i) {
    c.ready[i] = 0;
  }
  return c;
}

CloseSeries closes(std::vector<double> v) {
  CloseSeries c;
  static_cast<Column &>(c) = col(std::move(v));
  return c;
}

std::optional<double> opt(const Column &c, std::size_t i) {
  return c.ready[i] ? std::optional<double>{c.samples[i]} : std::nullopt;
}

} // namespace

TEST(ConsensusGates, BaselineCrossSeriesMatchesSample) {
  const CloseSeries close = closes({5, 4, 3, 6, 7, 2, 1});
  const Column base = col({4, 4, 4, 4, 4, 4, 4}, 1);
  const auto batch = BaselineCross::evaluate(close, base);
  BaselineCross live;
  for (std::size_t i = 0; i < batch.size(); ++i) {
    EXPECT_EQ(batch[i], live.apply(close.samples[i], opt(base, i))) << i;
  }
  EXPECT_EQ(batch[3], Cross::up);
  EXPECT_EQ(batch[5], Cross::down);
}

TEST(ConsensusGates, HigherTrendSeriesMatchesSample) {
  const CloseSeries close = closes({10, 10, 10, 3});
  const Column regime = col({8, 8, 12, 8}, 1);
  const Column macro = col({6, 6, 6, 6});
  const auto batch = HigherTrend::evaluate(close, regime, macro);
  for (std::size_t i = 0; i < batch.size(); ++i) {
    EXPECT_EQ(batch[i] != 0, HigherTrend::apply(close.samples[i],
                                                opt(regime, i), opt(macro, i)));
  }
  EXPECT_EQ(batch, (std::vector<char>{0, 1, 0, 0}));
}

TEST(ConsensusGates, RsiVoteSeriesMatchesSample) {
  const Column rsi = col({55, 51, 52, 49, 47, 50}, 0);
  const auto batch = RsiVote::evaluate(rsi);
  RsiVote live;
  for (std::size_t i = 0; i < batch.size(); ++i) {
    EXPECT_EQ(batch[i], live.apply(opt(rsi, i))) << i;
  }
  EXPECT_EQ(batch[0], Vote::bull);
  EXPECT_EQ(batch[1], Vote::neutral);
  EXPECT_EQ(batch[3], Vote::bear);
}

TEST(ConsensusGates, VolumeRatioSeriesMatchesSample) {
  VolumeSeries vol;
  static_cast<Column &>(vol) = col({12, 8, 10, 0});
  const Column mean = col({10, 10, 10, 10}, 0);
  const auto batch = VolumeRatio::evaluate(vol, mean);
  for (std::size_t i = 0; i < batch.size(); ++i) {
    EXPECT_EQ(batch[i], VolumeRatio::apply(vol.samples[i], opt(mean, i)));
  }
  EXPECT_EQ(batch[0], Vote::bull);
  EXPECT_EQ(batch[1], Vote::bear);
  EXPECT_EQ(batch[2], Vote::neutral);
  EXPECT_EQ(batch[3], Vote::neutral);
}

TEST(ConsensusGates, MacdSignSeriesMatchesSample) {
  const Column hist = col({1, -1, 1, 1}, 0);
  Column h = hist;
  h.ready[2] = 0;
  const std::vector<Vote> mom{Vote::bear, Vote::bull, Vote::bull,
                              Vote::neutral};
  const auto batch = MacdSign::evaluate(h, mom);
  for (std::size_t i = 0; i < batch.size(); ++i) {
    EXPECT_EQ(batch[i] != 0, MacdSign::apply(opt(h, i), mom[i]));
  }
  EXPECT_EQ(batch, (std::vector<char>{1, 0, 1, 1}));
}

TEST(ConsensusGates, AtrStopSeriesMatchesSample) {
  const CloseSeries close = closes({10, 4, 9, 3});
  const Column base = col({10, 10, 10, 10}, 0);
  const Column atr = col({2, 2, 2, 2}, 0);
  const auto batch = AtrStop::evaluate(close, base, atr);
  for (std::size_t i = 0; i < batch.size(); ++i) {
    EXPECT_EQ(batch[i] != 0,
              AtrStop::apply(close.samples[i], opt(base, i), opt(atr, i)));
  }
  EXPECT_EQ(batch, (std::vector<char>{0, 1, 0, 1}));
}

TEST(ConsensusParams, RsiBullChangesVote) {
  ConsensusParams p;
  EXPECT_EQ(RsiVote{p}.apply(53.0), Vote::bull);
  p.rsi_bull = 60.0;
  EXPECT_EQ(RsiVote{p}.apply(53.0), Vote::neutral);
  EXPECT_EQ(RsiVote{p}.apply(61.0), Vote::bull);
}

TEST(ConsensusParams, RsiBearChangesVote) {
  ConsensusParams p;
  EXPECT_EQ(RsiVote{p}.apply(47.0), Vote::bear);
  p.rsi_bear = 40.0;
  EXPECT_EQ(RsiVote{p}.apply(47.0), Vote::neutral);
  EXPECT_EQ(RsiVote{p}.apply(39.0), Vote::bear);
}

TEST(ConsensusParams, VolRatioBullChangesVote) {
  ConsensusParams p;
  EXPECT_EQ(VolumeRatio::apply(1.2, 1.0, p), Vote::bull);
  p.vol_ratio_bull = 1.5;
  EXPECT_EQ(VolumeRatio::apply(1.2, 1.0, p), Vote::neutral);
}

TEST(ConsensusParams, VolRatioBearChangesVote) {
  ConsensusParams p;
  EXPECT_EQ(VolumeRatio::apply(0.8, 1.0, p), Vote::bear);
  p.vol_ratio_bear = 0.5;
  EXPECT_EQ(VolumeRatio::apply(0.8, 1.0, p), Vote::neutral);
}

TEST(ConsensusParams, AtrStopMultChangesStop) {
  ConsensusParams p;
  EXPECT_FALSE(AtrStop::apply(96.0, 100.0, 2.0, p)); // stop at 95
  p.atr_stop_mult = 1.0;
  EXPECT_TRUE(AtrStop::apply(96.0, 100.0, 2.0, p)); // stop at 98
}

} // namespace dagobert
