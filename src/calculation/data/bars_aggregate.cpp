#include "bars_aggregate.hpp"

#include <algorithm>
#include <cstddef>

namespace dagobert {

std::vector<Bar> aggregate_bars(std::span<const Bar> source,
                                std::int64_t source_interval_ms,
                                std::int64_t duration_ms) {
  std::vector<Bar> out;
  if (source_interval_ms <= 0 || duration_ms <= 0 ||
      duration_ms % source_interval_ms != 0) {
    return out;
  }
  const auto steps = static_cast<std::size_t>(duration_ms / source_interval_ms);
  for (std::size_t i = 0; i + steps <= source.size(); ++i) {
    const std::int64_t open_ms = source[i].open_ms;
    if (open_ms % duration_ms != 0) {
      continue;
    }
    bool complete = true;
    for (std::size_t k = 1; k < steps && complete; ++k) {
      complete = source[i + k].open_ms ==
                 open_ms + static_cast<std::int64_t>(k) * source_interval_ms;
    }
    if (!complete) {
      continue;
    }
    Bar bar;
    bar.open_ms = open_ms;
    bar.open = source[i].open;
    bar.high = source[i].high;
    bar.low = source[i].low;
    bar.duration_ms = duration_ms;
    bar.segment_close.reserve(steps);
    double volume = 0.0;
    for (std::size_t k = 0; k < steps; ++k) {
      const Bar &c = source[i + k];
      bar.high = std::max(bar.high, c.high);
      bar.low = std::min(bar.low, c.low);
      volume += c.volume;
      bar.segment_close.push_back(c.close);
    }
    bar.close = source[i + steps - 1].close;
    bar.volume = volume;
    out.push_back(std::move(bar));
    i += steps - 1;
  }
  return out;
}

} // namespace dagobert
