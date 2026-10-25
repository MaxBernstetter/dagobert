#pragma once

#include "types.hpp"

#include <cstddef>
#include <cstdint>
#include <span>

namespace dagobert {

class Position;

[[nodiscard]] CloseSeries closes_of(std::span<const Bar> bars);
[[nodiscard]] VolumeSeries volumes_of(std::span<const Bar> bars);

[[nodiscard]] std::size_t bars_for(std::int32_t hours,
                                   std::int64_t bar_duration_ms);

void require_finite_close(const Bar &bar);

[[nodiscard]] StrategyResult strategy_result(const Position &book, double mark);

} // namespace dagobert
