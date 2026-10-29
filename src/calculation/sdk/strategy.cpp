#include "strategy.hpp"

#include "exit_policy.hpp"

#include <iostream>

namespace dagobert {

void Strategy::require_schema_version(
    const tradingcopilot::v1::RunEnvelope &envelope) {
  if (envelope.schema_version() != kSchemaVersion) {
    std::cerr << "schema_version must be " << kSchemaVersion << ": "
              << envelope.schema_version() << '\n';
    exit_config();
  }
}

Strategy::Strategy(const tradingcopilot::v1::RunEnvelope &envelope)
    : initial_quote_(Quote::require(envelope.initial_quote())) {
  require_schema_version(envelope);
}

} // namespace dagobert
