#pragma once

#include "sizing.hpp"

#include "tradingcopilot/v1/strategy.pb.h"

#include <optional>

namespace dagobert {

// Maps the proto `Sizing` block to a SizingConfig. Unset fields keep their
// defaults. An invalid value prints the violated range and exits 2.
[[nodiscard]] sizing::SizingConfig
sizing_from_proto(const tradingcopilot::v1::Sizing &proto);

} // namespace dagobert
