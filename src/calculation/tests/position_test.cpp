#include <gtest/gtest.h>

#include "position.hpp"
#include "quote.hpp"

namespace dagobert {
namespace {

enum class TestRule { round_trip_entry, round_trip_exit, illegal_entry };

Bar make_bar(std::int64_t open_ms, double close) {
  Bar bar;
  bar.open_ms = open_ms;
  bar.open = close;
  bar.high = close;
  bar.low = close;
  bar.close = close;
  bar.volume = 1.0;
  bar.duration_ms = 3'600'000;
  return bar;
}

void buy_while_long() {
  Position book{Quote::require(100.0)};
  const Bar first = make_bar(1'000, 10.0);
  book.buy_all(first, TestRule::round_trip_entry);
  book.buy_all(first, TestRule::illegal_entry);
}

void sell_while_flat() {
  Position book{Quote::require(100.0)};
  book.sell_all(make_bar(2'000, 10.0), TestRule::round_trip_exit);
}

} // namespace

TEST(Position, RoundTripAllInAtClose) {
  Position book{Quote::require(1'000.0)};
  const Bar entry = make_bar(1'000, 10.0);
  const Fill buy = book.buy_all(entry, TestRule::round_trip_entry);
  EXPECT_DOUBLE_EQ(buy.price, 10.0);
  EXPECT_DOUBLE_EQ(buy.qty, 100.0);
  EXPECT_EQ(buy.side, "buy");
  EXPECT_EQ(buy.reason, "round_trip_entry");

  const Bar exit_bar = make_bar(2'000, 20.0);
  const Fill sell = book.sell_all(exit_bar, TestRule::round_trip_exit);
  EXPECT_DOUBLE_EQ(sell.price, 20.0);
  EXPECT_DOUBLE_EQ(sell.qty, 100.0);
  EXPECT_EQ(sell.side, "sell");
  EXPECT_TRUE(book.flat());
  EXPECT_DOUBLE_EQ(book.cash(), 2'000.0);
  EXPECT_EQ(book.actions().size(), 2U);
  EXPECT_EQ(book.fills().size(), 2U);
}

TEST(Position, BuyAllWhileLongExits) {
  EXPECT_EXIT(buy_while_long(), ::testing::ExitedWithCode(1),
              "buy_all while long");
}

TEST(Position, SellAllWhileFlatExits) {
  EXPECT_EXIT(sell_while_flat(), ::testing::ExitedWithCode(1),
              "sell_all while flat");
}

} // namespace dagobert
