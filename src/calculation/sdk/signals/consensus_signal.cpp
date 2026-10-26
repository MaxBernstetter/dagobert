#include "consensus_signal.hpp"

#include "position.hpp"

#include <algorithm>
#include <utility>

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
                                    bool atr_stop, bool valid,
                                    std::optional<double> atr) {
  ConsensusGate g;
  if (!valid) {
    return g;
  }
  g.valid = true;
  g.atr = atr;
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
                          stop[i], valid, at(in.atr, i));
  }
  return out;
}

ConsensusSignal::ConsensusSignal(std::optional<sizing::SizingConfig> sizing)
    : sizing_(std::move(sizing)) {
  if (sizing_) {
    sizing_->stop_atr_mult = kAtrStopMult;
  }
}

std::optional<ConsensusRule>
ConsensusSignal::decide_sized(const ConsensusGate &gate, double close,
                              double equity, bool flat, std::size_t i) {
  const sizing::SizingConfig &cfg = *sizing_;
  if (!gate.valid) {
    return std::nullopt;
  }
  const bool has_atr = gate.atr.has_value();
  const double atr = gate.atr.value_or(0.0);
  if (flat) {
    if (!gate.entry || !has_atr ||
        (last_sell_ && i - *last_sell_ < kMinBarsFlatAfterSell)) {
      return std::nullopt;
    }
    const double n = sizing::notional(equity, atr, close, cfg);
    const double stop = sizing::stop_price(close, atr, cfg);
    if (!(n > 0.0) || sizing::below_min_order(n, cfg)) {
      return std::nullopt;
    }
    plan_ = Plan{.notional = n, .stop = stop, .equity = equity, .atr = atr};
    return ConsensusRule::consensus_buy;
  }
  if (units_.empty()) {
    return std::nullopt;
  }
  double held = 0.0;
  double stop = units_.front().stop;
  for (const sizing::Unit &u : units_) {
    held += u.qty;
    stop = std::max(stop, u.stop);
  }
  if (close <= stop) {
    return ConsensusRule::atr_stop;
  }
  if (gate.cross_down &&
      (!last_buy_ || i - *last_buy_ >= kMinBarsLongBeforeSell)) {
    return ConsensusRule::consensus_sell;
  }
  if (!has_atr) {
    return std::nullopt;
  }
  if (const double q = sizing::excess_base_for_heat(held, atr, equity, cfg);
      q > 0.0) {
    if (sizing::below_min_order(q * close, cfg)) {
      return std::nullopt;
    }
    plan_ = Plan{.qty = q};
    return ConsensusRule::vol_cut;
  }
  if (const auto n = sizing::pyramid_allowed(units_, units_.back().fill, close,
                                             equity, atr, cfg)) {
    if (sizing::below_min_order(*n, cfg)) {
      return std::nullopt;
    }
    const double new_stop = sizing::stop_price(close, atr, cfg);
    plan_ = Plan{.notional = *n,
                 .stop = new_stop,
                 .raise_by = std::max(0.0, cfg.add_mult() * atr),
                 .equity = equity,
                 .atr = atr};
    return ConsensusRule::pyramid;
  }
  return std::nullopt;
}

std::optional<ConsensusRule> ConsensusSignal::decide(const ConsensusGate &gate,
                                                     double close,
                                                     double equity, bool flat) {
  const std::size_t i = bar_++;
  std::optional<ConsensusRule> rule;
  if (sizing_) {
    rule = decide_sized(gate, close, equity, flat, i);
  } else if (flat) {
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
  if (rule == ConsensusRule::consensus_buy || rule == ConsensusRule::pyramid) {
    last_buy_ = i;
  } else if (rule) {
    last_sell_ = i;
  }
  return rule;
}

std::vector<Action>
ConsensusSignal::actions_from(Position &account, std::span<const Bar> bars,
                              const ConsensusSeries &mask,
                              std::optional<sizing::SizingConfig> sizing) {
  std::vector<Action> out;
  ConsensusSignal live{std::move(sizing)};
  const std::size_t count = std::min(bars.size(), mask.gate.size());
  for (std::size_t i = 0; i < count; ++i) {
    const auto rule =
        live.decide(mask.gate[i], bars[i].close, account.equity(bars[i].close),
                    account.flat());
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
  return decide(
      combine(cross, trend, macd, momentum, volume, stop, valid, in.atr),
      in.close, 0.0, flat);
}

std::optional<ConsensusRule> ConsensusSignal::apply(const ConsensusSample &in,
                                                    const Position &account) {
  const Cross cross = cross_.apply(in.close, in.signal);
  const Vote momentum = rsi_vote_.apply(in.rsi);
  const bool trend = HigherTrend::apply(in.close, in.regime, in.macro);
  const Vote volume = VolumeRatio::apply(in.volume, in.vol_mean);
  const bool macd = MacdSign::apply(in.hist, momentum);
  const bool stop = AtrStop::apply(in.close, in.signal, in.atr);
  const bool valid = in.signal.has_value() && in.rsi.has_value();
  return decide(
      combine(cross, trend, macd, momentum, volume, stop, valid, in.atr),
      in.close, account.equity(in.close), account.flat());
}

void ConsensusSignal::buy_unit(Position &account, const Bar &bar,
                               ConsensusRule rule) {
  const double spend = std::min(plan_.notional, account.cash());
  if (!(spend > 0.0) || sizing::below_min_order(spend, *sizing_)) {
    return;
  }
  const Fill fill = account.buy_notional(bar, spend, plan_.stop, rule);
  const sizing::RiskPct risk =
      sizing::risk_pct(spend, plan_.equity, plan_.atr, bar.close, *sizing_);
  account.annotate_last_risk(risk.requested, risk.capped);
  for (sizing::Unit &u : units_) {
    u.stop += plan_.raise_by;
  }
  units_.push_back({.fill = bar.close, .stop = plan_.stop, .qty = fill.qty});
}

void ConsensusSignal::execute(Position &account, const Bar &bar,
                              ConsensusRule rule) {
  if (!sizing_) {
    if (rule == ConsensusRule::consensus_buy) {
      account.buy_all(bar, rule);
      return;
    }
    account.sell_all(bar, rule);
    return;
  }
  switch (rule) {
  case ConsensusRule::consensus_buy:
  case ConsensusRule::pyramid:
    buy_unit(account, bar, rule);
    return;
  case ConsensusRule::vol_cut: {
    const double before = account.qty();
    account.sell_base(bar, plan_.qty, rule);
    if (account.flat()) {
      units_.clear();
      return;
    }
    const double keep = account.qty() / before;
    for (sizing::Unit &u : units_) {
      u.qty *= keep;
    }
    return;
  }
  case ConsensusRule::consensus_sell:
  case ConsensusRule::atr_stop:
    account.sell_all(bar, rule);
    units_.clear();
    return;
  }
}

void ConsensusSignal::reset() { *this = ConsensusSignal{sizing_}; }

} // namespace dagobert
