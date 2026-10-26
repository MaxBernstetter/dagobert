#pragma once

#include "sizing.hpp"

#include "tradingcopilot/v1/run_report.pb.h"
#include "tradingcopilot/v1/strategy.pb.h"
#include "types.hpp"

#include <optional>
#include <vector>

namespace dagobert {

// Maps the proto `Sizing` block to a SizingConfig. Unset fields keep their
// defaults. An invalid value prints the violated range and exits 2.
[[nodiscard]] sizing::SizingConfig
sizing_from_proto(const tradingcopilot::v1::Sizing &proto);

// Appends one ActionSizing per action (docs/v2/features/position-sizing.md).
void write_sizing_output(const std::vector<Action> &actions,
                         tradingcopilot::v1::RunReport &report);

} // namespace dagobert
