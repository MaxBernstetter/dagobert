#include <gtest/gtest.h>

#include "quote.hpp"

namespace dagobert {
namespace {

void require_quote(double amount) { Quote::require(amount); }

} // namespace

TEST(QuoteRequire, RejectsZero) {
  EXPECT_EXIT(require_quote(0.0), ::testing::ExitedWithCode(2),
              "Quote::require rejected initial quote: 0");
}

TEST(QuoteRequire, RejectsNegative) {
  EXPECT_EXIT(require_quote(-1.0), ::testing::ExitedWithCode(2),
              "Quote::require rejected initial quote: -1");
}

TEST(QuoteRequire, RejectsNonFinite) {
  EXPECT_EXIT(require_quote(1.0 / 0.0), ::testing::ExitedWithCode(2),
              "Quote::require rejected initial quote: inf");
}

} // namespace dagobert
