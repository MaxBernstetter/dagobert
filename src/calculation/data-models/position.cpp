#include "position.hpp"

namespace dagobert {

Position::Position(Quote initial_quote)
    : initial_quote_(initial_quote.amount()), cash_(initial_quote.amount()),
      qty_(0.0) {}

} // namespace dagobert
