#include "series.hpp"

#include "position.hpp"

#include <cmath>
#include <cstdlib>
#include <iostream>

namespace dagobert {

CloseSeries closes_of(std::span<const Bar> bars) {
  CloseSeries close;
  close.samples.reserve(bars.size());
  close.ready.reserve(bars.size());
  for (const Bar &bar : bars) {
    close.samples.push_back(bar.close);
    close.ready.push_back(1);
  }
  return close;
}

VolumeSeries volumes_of(std::span<const Bar> bars) {
  VolumeSeries volume;
  volume.samples.reserve(bars.size());
  volume.ready.reserve(bars.size());
  for (const Bar &bar : bars) {
    volume.samples.push_back(bar.volume);
    volume.ready.push_back(1);
  }
  return volume;
}

std::size_t bars_for(std::int32_t hours, std::int64_t bar_duration_ms) {
  if (hours <= 0 || bar_duration_ms <= 0) {
    std::cerr << "bars_for: hours and bar_duration_ms must be positive: hours="
              << hours << " bar_duration_ms=" << bar_duration_ms << '\n';
    std::exit(2);
  }
  const std::int64_t bars =
      static_cast<std::int64_t>(hours) * 3'600'000 / bar_duration_ms;
  if (bars <= 0) {
    std::cerr << "bars_for: bar count must be positive for hours=" << hours
              << " bar_duration_ms=" << bar_duration_ms << '\n';
    std::exit(2);
  }
  return static_cast<std::size_t>(bars);
}

void require_finite_close(const Bar &bar) {
  if (!std::isfinite(bar.close)) {
    std::cerr << "bar close must be finite at open_ms=" << bar.open_ms << '\n';
    std::exit(1);
  }
}

StrategyResult strategy_result(const Position &book, double mark) {
  StrategyResult result;
  result.cash = book.cash();
  result.qty = book.qty();
  result.equity = book.equity(mark);
  result.actions = book.actions();
  result.fills = book.fills();
  return result;
}

} // namespace dagobert
