#include "sizing.hpp"

#include <algorithm>
#include <cmath>
#include <initializer_list>

namespace dagobert::sizing {
namespace {

bool usable(std::initializer_list<double> values) {
  return std::ranges::all_of(
      values, [](double v) { return v > 0.0 && std::isfinite(v); });
}

double open_risk(const std::vector<Unit> &units) {
  double risk = 0.0;
  for (const Unit &u : units) {
    risk += std::max(0.0, u.qty * (u.fill - u.stop));
  }
  return risk;
}

} // namespace

std::string validate(const SizingConfig &cfg) {
  const double unit = cfg.unit_risk();
  const double heat = cfg.heat();
  if (!(unit >= 0.25 && unit <= 2.0)) {
    return "unitRiskPct must be in [0.25, 2]";
  }
  if (!(heat >= unit && heat <= 2.0)) {
    return "heatPct must be in [unitRiskPct, 2]";
  }
  if (cfg.units() < 1 || cfg.units() > 4) {
    return "maxUnits must be in [1, 4]";
  }
  if (!(cfg.stop_mult() > 0.0)) {
    return "stopAtrMult must be positive";
  }
  if (!(cfg.add_mult() > 0.0)) {
    return "addAtrMult must be positive";
  }
  if (!(cfg.max_weight() > 0.0 && cfg.max_weight() <= 100.0)) {
    return "maxWeightPct must be in (0, 100]";
  }
  return "";
}

double notional(double equity, double atr, double fill,
                const SizingConfig &cfg) {
  if (!usable({equity, atr, fill})) {
    return 0.0;
  }
  const double by_risk =
      equity * cfg.unit_risk() / 100.0 / (cfg.stop_mult() * atr / fill);
  return std::min(by_risk, equity * cfg.max_weight() / 100.0);
}

RiskPct risk_pct(double notional, double equity, double atr, double fill,
                 const SizingConfig &cfg) {
  RiskPct out;
  out.requested = cfg.unit_risk();
  if (usable({equity, atr, fill, notional})) {
    out.capped = notional * (cfg.stop_mult() * atr / fill) / equity * 100.0;
  }
  return out;
}

double stop_price(double fill, double atr, const SizingConfig &cfg) {
  return fill - cfg.stop_mult() * atr;
}

double excess_base_for_heat(double base_held, double atr, double equity,
                            const SizingConfig &cfg) {
  if (!usable({base_held, atr, equity})) {
    return 0.0;
  }
  const double dist = cfg.stop_mult() * atr;
  const double over = base_held * dist - equity * cfg.heat() / 100.0;
  if (over <= 0.0) {
    return 0.0;
  }
  return std::min(over / dist, base_held);
}

std::vector<Unit> raise_stops(std::vector<Unit> units, double atr,
                              const SizingConfig &cfg) {
  if (!usable({atr})) {
    return units;
  }
  const double step = std::max(0.0, cfg.add_mult() * atr);
  for (Unit &u : units) {
    u.stop += step;
  }
  return units;
}

std::optional<double> pyramid_allowed(const std::vector<Unit> &units,
                                      double last_fill, double price,
                                      double equity, double atr,
                                      const SizingConfig &cfg) {
  if (units.empty() || static_cast<int>(units.size()) >= cfg.units() ||
      !usable({last_fill, price, equity, atr})) {
    return std::nullopt;
  }
  if (price - last_fill < cfg.add_mult() * atr) {
    return std::nullopt;
  }
  const double n = notional(equity, atr, price, cfg);
  if (n <= 0.0) {
    return std::nullopt;
  }
  const double new_risk = n * cfg.stop_mult() * atr / price;
  const double total = open_risk(raise_stops(units, atr, cfg)) + new_risk;
  if (total > equity * cfg.heat() / 100.0 * (1.0 + 1e-9)) {
    return std::nullopt;
  }
  return n;
}

} // namespace dagobert::sizing
