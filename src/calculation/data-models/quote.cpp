#include "quote.hpp"

#include <cmath>
#include "exit_policy.hpp"

#include <cstdlib>
#include <iostream>

namespace dagobert {

Quote::Quote(double amount) : amount_(amount) {}

Quote Quote::require(double amount) {
  if (!std::isfinite(amount) || amount <= 0.0) {
    std::cerr << "Quote::require rejected initial quote: " << amount << '\n';
    exit_config();
  }
  return Quote(amount);
}

} // namespace dagobert
