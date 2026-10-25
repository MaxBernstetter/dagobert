#include "sma_cross_signal.hpp"

#include "position.hpp"

#include <algorithm>

namespace dagobert {
namespace {

[[nodiscard]] std::optional<SmaCrossRule>
rule_at_index(const CloseSeries& fast, const CloseSeries& slow, std::size_t index) {
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

SmaCrossSeries SmaCrossSignal::evaluate(const CloseSeries& fast,
                                        const CloseSeries& slow) {
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

std::vector<Action> SmaCrossSignal::actions_from(Position& account,
                                                 std::span<const Bar> bars,
                                                 const SmaCrossSeries& mask) {
  std::vector<Action> out;
  const std::size_t count =
      std::min({bars.size(), mask.rule.size()});
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

std::optional<SmaCrossRule>
SmaCrossSignal::apply(std::optional<double> fast, std::optional<double> slow,
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

void SmaCrossSignal::execute(Position& account, const Bar& bar,
                             SmaCrossRule rule) {
  if (rule == SmaCrossRule::golden_cross) {
    account.buy_all(bar, rule);
    return;
  }
  account.sell_all(bar, rule);
}

void SmaCrossSignal::reset() { prev_above_.reset(); }

} // namespace dagobert
