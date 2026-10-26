#pragma once

#include <cstdlib>
#include <stdexcept>
#include <string>

namespace dagobert {

// Thrown instead of exiting 2 while a sweep builds one item's strategy, so a
// bad item config fails that item only.
class ConfigError : public std::runtime_error {
public:
  using std::runtime_error::runtime_error;
};

[[nodiscard]] inline bool &throw_on_config_error() {
  static bool enabled = false;
  return enabled;
}

// A config error: exit 2 (single run) or throw ConfigError (sweep). The
// caller has already written the reason to stderr.
[[noreturn]] inline void exit_config() {
  if (throw_on_config_error()) {
    throw ConfigError("invalid config");
  }
  std::exit(2);
}

} // namespace dagobert
