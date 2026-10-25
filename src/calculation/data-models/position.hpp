#pragma once

#include "quote.hpp"
#include "types.hpp"

#include <magic_enum/magic_enum.hpp>

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
  [[nodiscard]] const std::vector<Action>& actions() const { return actions_; }
  [[nodiscard]] const std::vector<Fill>& fills() const { return fills_; }

  template <typename Rule>
  Fill buy_all(const Bar& bar, Rule rule);

  template <typename Rule>
  Fill sell_all(const Bar& bar, Rule rule);

private:
  double initial_quote_;
  double cash_;
  double qty_;
  std::vector<Action> actions_;
  std::vector<Fill> fills_;
};

template <typename Rule>
Fill Position::buy_all(const Bar& bar, Rule rule) {
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

template <typename Rule>
Fill Position::sell_all(const Bar& bar, Rule rule) {
  if (flat()) {
    std::cerr << "sell_all while flat at bar_open_ms=" << bar.open_ms
              << " side=sell\n";
    std::exit(1);
  }
  const double proceeds = qty_ * bar.close;
  const double sold_qty = qty_;
  qty_ = 0.0;
  cash_ = proceeds;

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

} // namespace dagobert
