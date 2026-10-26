#pragma once

#include "types.hpp"

#include <cstdint>
#include <span>
#include <vector>

namespace dagobert {

// Rolls source candles (sorted by open time, `source_interval_ms` apart) up to
// bars of `duration_ms`, the way the technical-indicator job does. A bar opens
// at a multiple of `duration_ms` and exists only when every source candle of
// its window is present; incomplete windows are dropped. open/close come from
// the first/last candle, high/low are the extremes, volume is the sum, and
// segment_close holds each candle close. Returns nothing when `duration_ms` is
// not a positive multiple of `source_interval_ms`.
[[nodiscard]] std::vector<Bar> aggregate_bars(std::span<const Bar> source,
                                              std::int64_t source_interval_ms,
                                              std::int64_t duration_ms);

} // namespace dagobert
