#include "consensus_signal.hpp"

#include "position.hpp"

#include <algorithm>

namespace dagobert {
namespace {

[[nodiscard]] std::optional<double> at(const Column &col, std::size_t i) {
  if (i >= col.samples.size() || i >= col.ready.size() || !col.ready[i]) {
    return std::nullopt;
  }
  return col.samples[i];
}

[[nodiscard]] ConsensusGate combine(Cross cross, bool higher_trend,
                                    bool macd_ok, Vote momentum, Vote volume,
                                    bool atr_stop, bool valid) {
  ConsensusGate g;
  if (!valid) {
    return g;
  }
  g.entry = cross == Cross::up && higher_trend && macd_ok &&
            momentum != Vote::bear && volume != Vote::bear;
  g.cross_down = cross == Cross::down;
  g.atr_stop = atr_stop;
  return g;
}

} // namespace

ConsensusSeries ConsensusSignal::evaluate(const ConsensusColumns &in) {
  const std::size_t count = in.close.samples.size();
  ConsensusSeries out;
  out.gate.assign(count, ConsensusGate{});
  const std::vector<Cross> cross = BaselineCross::evaluate(in.close, in.signal);
  const std::vector<char> trend =
      HigherTrend::evaluate(in.close, in.regime, in.macro);
  const std::vector<Vote> momentum = RsiVote::evaluate(in.rsi);
  const std::vector<Vote> volume =
      VolumeRatio::evaluate(in.volume, in.vol_mean);
  const std::vector<char> macd = MacdSign::evaluate(in.hist, momentum);
  const std::vector<char> stop = AtrStop::evaluate(in.close, in.signal, in.atr);
  for (std::size_t i = 0; i < count; ++i) {
    if (i >= cross.size() || i >= trend.size() || i >= momentum.size() ||
        i >= volume.size() || i >= macd.size() || i >= stop.size()) {
      break;
    }
    const bool valid = in.close.ready[i] && at(in.signal, i) && at(in.rsi, i);
    out.gate[i] = combine(cross[i], trend[i], macd[i], momentum[i], volume[i],
                          stop[i], valid);
  }
  return out;
}

std::optional<ConsensusRule> ConsensusSignal::decide(const ConsensusGate &gate,
                                                     bool flat) {
  const std::size_t i = bar_++;
  std::optional<ConsensusRule> rule;
  if (flat) {
    if (gate.entry &&
        (!last_sell_ || i - *last_sell_ >= kMinBarsFlatAfterSell)) {
      rule = ConsensusRule::consensus_buy;
    }
  } else if (!last_buy_ || i - *last_buy_ >= kMinBarsLongBeforeSell) {
    if (gate.cross_down) {
      rule = ConsensusRule::consensus_sell;
    } else if (gate.atr_stop) {
      rule = ConsensusRule::atr_stop;
    }
  }
  if (rule == ConsensusRule::consensus_buy) {
    last_buy_ = i;
  } else if (rule) {
    last_sell_ = i;
  }
  return rule;
}

std::vector<Action> ConsensusSignal::actions_from(Position &account,
                                                  std::span<const Bar> bars,
                                                  const ConsensusSeries &mask) {
  std::vector<Action> out;
  ConsensusSignal live;
  const std::size_t count = std::min(bars.size(), mask.gate.size());
  for (std::size_t i = 0; i < count; ++i) {
    const auto rule = live.decide(mask.gate[i], account.flat());
    if (rule) {
      live.execute(account, bars[i], *rule);
      out.push_back(account.actions().back());
    }
  }
  return out;
}

std::optional<ConsensusRule> ConsensusSignal::apply(const ConsensusSample &in,
                                                    bool flat) {
  const Cross cross = cross_.apply(in.close, in.signal);
  const Vote momentum = rsi_vote_.apply(in.rsi);
  const bool trend = HigherTrend::apply(in.close, in.regime, in.macro);
  const Vote volume = VolumeRatio::apply(in.volume, in.vol_mean);
  const bool macd = MacdSign::apply(in.hist, momentum);
  const bool stop = AtrStop::apply(in.close, in.signal, in.atr);
  const bool valid = in.signal.has_value() && in.rsi.has_value();
  return decide(combine(cross, trend, macd, momentum, volume, stop, valid),
                flat);
}

void ConsensusSignal::execute(Position &account, const Bar &bar,
                              ConsensusRule rule) {
  if (rule == ConsensusRule::consensus_buy) {
    account.buy_all(bar, rule);
    return;
  }
  account.sell_all(bar, rule);
}

void ConsensusSignal::reset() { *this = ConsensusSignal{}; }

} // namespace dagobert
