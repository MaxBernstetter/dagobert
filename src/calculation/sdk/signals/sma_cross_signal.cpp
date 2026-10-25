#include "sma_cross_signal.hpp"

#include "position.hpp"

#include <algorithm>
#include <cmath>
#include <limits>

namespace dagobert {
namespace {

[[nodiscard]] std::optional<SmaCrossRule> rule_at_index(const CloseSeries &fast,
                                                        const CloseSeries &slow,
                                                        std::size_t index) {
  if (index == 0U || !fast.ready[index] || !slow.ready[index] ||
      !fast.ready[index - 1U] || !slow.ready[index - 1U]) {
    return std::nullopt;
  }
  const bool above = fast.samples[index] > slow.samples[index];
  const bool prev_above = fast.samples[index - 1U] > slow.samples[index - 1U];
  if (!prev_above && above) {
    return SmaCrossRule::golden_cross;
  }
  if (prev_above && !above) {
    return SmaCrossRule::death_cross;
  }
  return std::nullopt;
}

} // namespace

SmaCrossSeries SmaCrossSignal::evaluate(const CloseSeries &fast,
                                        const CloseSeries &slow) {
  SmaCrossSeries mask;
  const std::size_t count = fast.samples.size();
  mask.rule.assign(count, std::nullopt);
  if (slow.samples.size() != count) {
    return mask;
  }
  for (std::size_t i = 0; i < count; ++i) {
    mask.rule[i] = rule_at_index(fast, slow, i);
  }
  return mask;
}

std::vector<Action> SmaCrossSignal::actions_from(Position &account,
                                                 std::span<const Bar> bars,
                                                 const SmaCrossSeries &mask) {
  std::vector<Action> out;
  const std::size_t count = std::min({bars.size(), mask.rule.size()});
  for (std::size_t i = 0; i < count; ++i) {
    if (!mask.rule[i].has_value()) {
      continue;
    }
    const SmaCrossRule rule = *mask.rule[i];
    if (rule == SmaCrossRule::golden_cross && account.flat()) {
      account.buy_all(bars[i], rule);
      out.push_back(account.actions().back());
    } else if (rule == SmaCrossRule::death_cross && !account.flat()) {
      account.sell_all(bars[i], rule);
      out.push_back(account.actions().back());
    }
  }
  return out;
}

std::optional<SmaCrossRule> SmaCrossSignal::apply(std::optional<double> fast,
                                                  std::optional<double> slow,
                                                  bool flat) {
  if (!fast.has_value() || !slow.has_value()) {
    return std::nullopt;
  }
  const bool above = *fast > *slow;
  if (!prev_above_.has_value()) {
    prev_above_ = above;
    return std::nullopt;
  }
  std::optional<SmaCrossRule> rule;
  if (!*prev_above_ && above) {
    rule = SmaCrossRule::golden_cross;
  } else if (*prev_above_ && !above) {
    rule = SmaCrossRule::death_cross;
  }
  prev_above_ = above;
  if (rule == SmaCrossRule::death_cross && flat) {
    return std::nullopt;
  }
  return rule;
}

void SmaCrossSignal::execute(Position &account, const Bar &bar,
                             SmaCrossRule rule) {
  if (rule == SmaCrossRule::golden_cross) {
    account.buy_all(bar, rule);
    return;
  }
  account.sell_all(bar, rule);
}

void SmaCrossSignal::step_sized(Position &account, const Bar &bar,
                                std::optional<double> fast,
                                std::optional<double> slow,
                                std::optional<double> atr_value) {
  const sizing::SizingConfig &sz = *sizing_;
  const auto cross = apply(fast, slow, false);
  const bool bullish = cross == SmaCrossRule::golden_cross;
  const bool bearish = cross == SmaCrossRule::death_cross;
  const double price = bar.close;
  const bool atr_ok = atr_value.has_value() && *atr_value > 0.0;
  const double atr = atr_ok ? *atr_value : 0.0;
  const double equity = account.equity(price);

  if (account.flat()) {
    units_.clear();
    if (!bullish || !atr_ok || !(price > 0.0)) {
      return;
    }
    const double spend =
        std::min(sizing::notional(equity, atr, price, sz), account.cash());
    const double stop = sizing::stop_price(price, atr, sz);
    if (!(spend > 0.0) || !(stop > 0.0) || !(stop < price)) {
      return;
    }
    const double before = account.qty();
    account.buy_notional(bar, spend, stop, SmaCrossRule::golden_cross);
    units_.push_back({price, stop, account.qty() - before});
    return;
  }

  if (!units_.empty() && price > 0.0) {
    double top = -std::numeric_limits<double>::infinity();
    for (const sizing::Unit &u : units_) {
      top = std::max(top, u.stop);
    }
    if (price <= top) {
      account.sell_all(bar, SmaCrossRule::atr_stop);
      units_.clear();
      return;
    }
  }
  if (bearish) {
    account.sell_all(bar, SmaCrossRule::death_cross);
    units_.clear();
    return;
  }
  if (units_.empty() || !atr_ok || !(price > 0.0) || !(equity > 0.0)) {
    return;
  }

  double held = 0.0;
  for (const sizing::Unit &u : units_) {
    held += u.qty;
  }
  if (const double cut = sizing::excess_base_for_heat(held, atr, equity, sz);
      cut > 0.0) {
    const double before = account.qty();
    account.sell_base(bar, cut, SmaCrossRule::vol_cut);
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

  const auto add = sizing::pyramid_allowed(units_, units_.back().fill, price,
                                           equity, atr, sz);
  if (!add.has_value()) {
    return;
  }
  const double spend = std::min(*add, account.cash());
  const double stop = sizing::stop_price(price, atr, sz);
  if (!(spend > 0.0) || !(stop > 0.0) || !(stop < price)) {
    return;
  }
  const double step = sizing::raise_stops({units_.front()}, atr, sz)[0].stop -
                      units_.front().stop;
  const double before = account.qty();
  account.buy_notional(bar, spend, stop, SmaCrossRule::pyramid);
  for (sizing::Unit &u : units_) {
    u.stop += std::max(0.0, step);
  }
  units_.push_back({price, stop, account.qty() - before});
}

void SmaCrossSignal::reset() {
  prev_above_.reset();
  units_.clear();
}

} // namespace dagobert
