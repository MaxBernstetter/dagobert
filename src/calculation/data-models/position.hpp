#pragma once

#include "quote.hpp"
#include "types.hpp"

#include <magic_enum/magic_enum.hpp>

#include <algorithm>
#include <cstdlib>
#include <iostream>
#include <string>
#include <vector>

namespace dagobert {

class Position {
public:
  explicit Position(Quote initial_quote);

  [[nodiscard]] bool flat() const { return qty_ == 0.0; }
  [[nodiscard]] double initial_quote() const { return initial_quote_; }
  [[nodiscard]] double cash() const { return cash_; }
  [[nodiscard]] double qty() const { return qty_; }
  [[nodiscard]] double equity(double mark) const { return cash_ + qty_ * mark; }
  [[nodiscard]] const std::vector<Action> &actions() const { return actions_; }
  [[nodiscard]] const std::vector<Fill> &fills() const { return fills_; }

  template <typename Rule> Fill buy_all(const Bar &bar, Rule rule);

  template <typename Rule> Fill sell_all(const Bar &bar, Rule rule);

  // Spends `notional` of cash at the bar close with the given stop price.
  template <typename Rule>
  Fill buy_notional(const Bar &bar, double notional, double stop, Rule rule);

  // Sells `qty` base at the bar close.
  template <typename Rule>
  Fill sell_base(const Bar &bar, double qty, Rule rule);

private:
  template <typename Rule>
  Fill record(const Bar &bar, Side side, double qty, double weight_pct,
              double stop, Rule rule);

  double initial_quote_;
  double cash_;
  double qty_;
  std::vector<Action> actions_;
  std::vector<Fill> fills_;
};

template <typename Rule> Fill Position::buy_all(const Bar &bar, Rule rule) {
  if (!flat()) {
    std::cerr << "buy_all while long at bar_open_ms=" << bar.open_ms
              << " side=buy\n";
    std::exit(1);
  }
  const double qty = cash_ / bar.close;
  qty_ = qty;
  cash_ = 0.0;

  Action action;
  action.bar_open_ms = bar.open_ms;
  action.side = Side::buy;
  action.reason = std::string(magic_enum::enum_name(rule));
  action.weight_pct = 100.0;
  actions_.push_back(action);

  Fill fill;
  fill.ts_ms = bar.open_ms + bar.duration_ms;
  fill.side = std::string(magic_enum::enum_name(Side::buy));
  fill.qty = qty;
  fill.price = bar.close;
  fill.reason = action.reason;
  fills_.push_back(fill);
  return fill;
}

template <typename Rule> Fill Position::sell_all(const Bar &bar, Rule rule) {
  if (flat()) {
    std::cerr << "sell_all while flat at bar_open_ms=" << bar.open_ms
              << " side=sell\n";
    std::exit(1);
  }
  const double proceeds = qty_ * bar.close;
  const double sold_qty = qty_;
  qty_ = 0.0;
  cash_ += proceeds;

  Action action;
  action.bar_open_ms = bar.open_ms;
  action.side = Side::sell;
  action.reason = std::string(magic_enum::enum_name(rule));
  action.weight_pct = 100.0;
  actions_.push_back(action);

  Fill fill;
  fill.ts_ms = bar.open_ms + bar.duration_ms;
  fill.side = std::string(magic_enum::enum_name(Side::sell));
  fill.qty = sold_qty;
  fill.price = bar.close;
  fill.reason = action.reason;
  fills_.push_back(fill);
  return fill;
}

template <typename Rule>
Fill Position::record(const Bar &bar, Side side, double qty, double weight_pct,
                      double stop, Rule rule) {
  Action action;
  action.bar_open_ms = bar.open_ms;
  action.side = side;
  action.reason = std::string(magic_enum::enum_name(rule));
  action.weight_pct = weight_pct;
  action.stop = stop;
  actions_.push_back(action);

  Fill fill;
  fill.ts_ms = bar.open_ms + bar.duration_ms;
  fill.side = std::string(magic_enum::enum_name(side));
  fill.qty = qty;
  fill.price = bar.close;
  fill.reason = action.reason;
  fills_.push_back(fill);
  return fill;
}

template <typename Rule>
Fill Position::buy_notional(const Bar &bar, double notional, double stop,
                            Rule rule) {
  if (!(notional > 0.0) || notional > cash_ * (1.0 + 1e-9) ||
      !(stop < bar.close)) { // a stop <= 0 is valid: it never triggers
    std::cerr << "buy_notional invalid notional=" << notional
              << " stop=" << stop << " at bar_open_ms=" << bar.open_ms
              << " side=buy\n";
    std::exit(1);
  }
  const double spend = std::min(notional, cash_);
  const double equity_before = equity(bar.close);
  qty_ += spend / bar.close;
  cash_ -= spend;
  return record(bar, Side::buy, spend / bar.close,
                spend / equity_before * 100.0, stop, rule);
}

template <typename Rule>
Fill Position::sell_base(const Bar &bar, double qty, Rule rule) {
  if (!(qty > 0.0) || qty > qty_ * (1.0 + 1e-9)) {
    std::cerr << "sell_base invalid qty=" << qty
              << " at bar_open_ms=" << bar.open_ms << " side=sell\n";
    std::exit(1);
  }
  const double sold = std::min(qty, qty_);
  const double weight = sold * bar.close / equity(bar.close) * 100.0;
  qty_ = sold == qty_ ? 0.0 : qty_ - sold;
  cash_ += sold * bar.close;
  return record(bar, Side::sell, sold, weight, 0.0, rule);
}

} // namespace dagobert
