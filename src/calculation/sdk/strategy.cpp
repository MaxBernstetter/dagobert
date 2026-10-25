#include "strategy.hpp"

#include <cstdlib>
#include <iostream>

namespace dagobert {

void Strategy::require_schema_version(
    const tradingcopilot::v1::RunEnvelope &envelope) {
  if (envelope.schema_version() != 1U) {
    std::cerr << "schema_version must be 1: " << envelope.schema_version()
              << '\n';
    std::exit(2);
  }
}

Strategy::Strategy(const tradingcopilot::v1::RunEnvelope &envelope)
    : initial_quote_(Quote::require(envelope.initial_quote())) {
  require_schema_version(envelope);
}

} // namespace dagobert
