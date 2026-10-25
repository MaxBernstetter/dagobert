#pragma once

#include <optional>
#include <string>
#include <vector>

namespace dagobert::sizing {

// Optional `sizing` block. An unset field takes its documented default; an
// explicit value (including 0) is kept and checked by validate().
struct SizingConfig {
  std::optional<double> unit_risk_pct;
  std::optional<double> heat_pct;
  std::optional<int> max_units;
  std::optional<double> stop_atr_mult;
  std::optional<double> add_atr_mult;
  std::optional<double> max_weight_pct;
  // Smallest order in quote currency; smaller buys and partial sells are
  // skipped. Full exits ignore it.
  std::optional<double> min_order_quote;

  [[nodiscard]] double unit_risk() const { return unit_risk_pct.value_or(1.0); }
  [[nodiscard]] double heat() const { return heat_pct.value_or(2.0); }
  [[nodiscard]] int units() const { return max_units.value_or(1); }
  [[nodiscard]] double stop_mult() const { return stop_atr_mult.value_or(2.0); }
  [[nodiscard]] double add_mult() const { return add_atr_mult.value_or(0.5); }
  [[nodiscard]] double min_order() const {
    return min_order_quote.value_or(1.0);
  }
  [[nodiscard]] double max_weight() const {
    return max_weight_pct.value_or(100.0);
  }
};

// One open buy: fill price, stop price and base quantity.
struct Unit {
  double fill{};
  double stop{};
  double qty{};
};

struct RiskPct {
  double requested{};
  double capped{};
};

// True when an order worth `quote` is under the configured minimum.
[[nodiscard]] bool below_min_order(double quote, const SizingConfig &cfg);

// Empty string when valid, otherwise the first violated range.
[[nodiscard]] std::string validate(const SizingConfig &cfg);

// Quote spend for one unit; 0 when an input is unusable.
[[nodiscard]] double notional(double equity, double atr, double fill,
                              const SizingConfig &cfg);

// Requested risk and the risk percent of equity the notional carries at its
// stop after the weight cap (capped is 0 when an input is unusable).
[[nodiscard]] RiskPct risk_pct(double notional, double equity, double atr,
                               double fill, const SizingConfig &cfg);

[[nodiscard]] double stop_price(double fill, double atr,
                                const SizingConfig &cfg);

// Base to sell so the held base, stopped at stop_atr_mult * atr, risks at most
// heat_pct of equity. The stop is unchanged.
[[nodiscard]] double excess_base_for_heat(double base_held, double atr,
                                          double equity,
                                          const SizingConfig &cfg);

// Another unit is allowed at price; returns its notional, or nullopt.
[[nodiscard]] std::optional<double>
pyramid_allowed(const std::vector<Unit> &units, double last_fill, double price,
                double equity, double atr, const SizingConfig &cfg);

// Moves every stop up by add_atr_mult * atr; stops never move down.
[[nodiscard]] std::vector<Unit> raise_stops(std::vector<Unit> units, double atr,
                                            const SizingConfig &cfg);

} // namespace dagobert::sizing
