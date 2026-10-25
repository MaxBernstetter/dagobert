#pragma once

#include "types.hpp"

#include <cstdint>
#include <string>
#include <vector>

namespace dagobert {

[[nodiscard]] std::vector<Bar>
load_bars_from_lake(const std::string &data_dir, const std::string &provider,
                    const std::string &base, const std::string &quote,
                    std::int64_t from_ms, std::int64_t to_ms);

} // namespace dagobert
