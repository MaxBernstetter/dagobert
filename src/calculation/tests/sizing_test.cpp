#include <gtest/gtest.h>

#include "position.hpp"
#include "sizing.hpp"

#include <google/protobuf/struct.pb.h>
#include <google/protobuf/util/json_util.h>

#include <fstream>
#include <sstream>

namespace dagobert::sizing {
namespace {

using google::protobuf::ListValue;
using google::protobuf::Struct;

constexpr double kTol = 1e-9;

Struct load_vectors() {
  std::ifstream in(SIZING_VECTORS_PATH);
  std::stringstream text;
  text << in.rdbuf();
  Struct root;
  const auto status =
      google::protobuf::util::JsonStringToMessage(text.str(), &root);
  EXPECT_TRUE(status.ok()) << status.message();
  return root;
}

double num(const Struct &s, const char *key) {
  return s.fields().at(key).number_value();
}

SizingConfig cfg_of(const Struct &s) {
  SizingConfig c;
  const auto it = s.fields().find("cfg");
  if (it == s.fields().end()) {
    return c;
  }
  const Struct &f = it->second.struct_value();
  const auto get = [&f](const char *k) -> std::optional<double> {
    const auto v = f.fields().find(k);
    if (v == f.fields().end()) {
      return std::nullopt;
    }
    return v->second.number_value();
  };
  c.unit_risk_pct = get("unitRiskPct");
  c.heat_pct = get("heatPct");
  c.stop_atr_mult = get("stopAtrMult");
  c.add_atr_mult = get("addAtrMult");
  c.max_weight_pct = get("maxWeightPct");
  if (const auto m = get("maxUnits")) {
    c.max_units = static_cast<int>(*m);
  }
  return c;
}

const ListValue &list(const Struct &root, const char *key) {
  return root.fields().at(key).list_value();
}

Bar make_bar(std::int64_t open_ms, double close) {
  Bar bar;
  bar.open_ms = open_ms;
  bar.open = bar.high = bar.low = bar.close = close;
  bar.volume = 1.0;
  bar.duration_ms = 3'600'000;
  return bar;
}

enum class TestRule { entry, add, trim };

} // namespace

TEST(Sizing, NotionalVectors) {
  const Struct root = load_vectors();
  for (const auto &v : list(root, "notional").values()) {
    const Struct &c = v.struct_value();
    const SizingConfig cfg = cfg_of(c);
    const double n =
        notional(num(c, "equity"), num(c, "atr"), num(c, "fill"), cfg);
    EXPECT_NEAR(n, num(c, "want"), kTol)
        << c.fields().at("name").string_value();
    const RiskPct r =
        risk_pct(n, num(c, "equity"), num(c, "atr"), num(c, "fill"), cfg);
    EXPECT_NEAR(r.capped, num(c, "wantRiskPct"), kTol);
    EXPECT_NEAR(r.requested, cfg.unit_risk(), kTol);
  }
}

TEST(Sizing, StopPriceVectors) {
  const Struct root = load_vectors();
  for (const auto &v : list(root, "stopPrice").values()) {
    const Struct &c = v.struct_value();
    EXPECT_NEAR(stop_price(num(c, "fill"), num(c, "atr"), cfg_of(c)),
                num(c, "want"), kTol);
  }
}

TEST(Sizing, ExcessBaseVectors) {
  const Struct root = load_vectors();
  for (const auto &v : list(root, "excessBase").values()) {
    const Struct &c = v.struct_value();
    EXPECT_NEAR(excess_base_for_heat(num(c, "baseHeld"), num(c, "atr"),
                                     num(c, "equity"), cfg_of(c)),
                num(c, "want"), kTol)
        << c.fields().at("name").string_value();
  }
}

TEST(Sizing, PyramidVectors) {
  const Struct root = load_vectors();
  for (const auto &v : list(root, "pyramid").values()) {
    const Struct &c = v.struct_value();
    std::vector<Unit> units;
    for (const auto &u : c.fields().at("units").list_value().values()) {
      const Struct &us = u.struct_value();
      units.push_back({num(us, "fill"), num(us, "stop"), num(us, "qty")});
    }
    const auto got =
        pyramid_allowed(units, num(c, "lastFill"), num(c, "price"),
                        num(c, "equity"), num(c, "atr"), cfg_of(c));
    EXPECT_EQ(got.has_value(), c.fields().at("want").bool_value())
        << c.fields().at("name").string_value();
  }
}

TEST(Sizing, WeightCapBindsAndRiskFallsBelowRequested) {
  const SizingConfig cfg;
  const double n = notional(100000, 400, 100000, cfg);
  EXPECT_NEAR(n, 100000, kTol);
  EXPECT_LT(risk_pct(n, 100000, 400, 100000, cfg).capped, cfg.unit_risk());
}

TEST(Sizing, RaiseStopsNeverMovesDown) {
  SizingConfig cfg;
  cfg.add_atr_mult = 0.5;
  const auto up = raise_stops({{100, 96, 1}}, 2000, cfg);
  EXPECT_NEAR(up[0].stop, 1096, kTol);
  const auto same = raise_stops({{100, 96, 1}}, -5, cfg);
  EXPECT_NEAR(same[0].stop, 96, kTol);
}

TEST(Sizing, ValidateRanges) {
  EXPECT_EQ(validate({}), "");
  SizingConfig bad;
  bad.unit_risk_pct = 0.0;
  EXPECT_NE(validate(bad), "");
  bad = {};
  bad.heat_pct = 0.5;
  EXPECT_NE(validate(bad), "");
  bad = {};
  bad.max_units = 5;
  EXPECT_NE(validate(bad), "");
  bad = {};
  bad.max_weight_pct = 101;
  EXPECT_NE(validate(bad), "");
}

TEST(SizingPosition, BuyNotionalAndSellBase) {
  Position book{Quote::require(1000.0)};
  const Fill buy =
      book.buy_notional(make_bar(1, 10.0), 400.0, 8.0, TestRule::entry);
  EXPECT_NEAR(buy.qty, 40.0, kTol);
  EXPECT_NEAR(book.cash(), 600.0, kTol);
  EXPECT_NEAR(book.actions().back().stop, 8.0, kTol);
  const Fill add =
      book.buy_notional(make_bar(2, 10.0), 100.0, 9.0, TestRule::add);
  EXPECT_NEAR(book.qty(), 50.0, kTol);
  EXPECT_EQ(add.side, "buy");
  const Fill trim = book.sell_base(make_bar(3, 20.0), 10.0, TestRule::trim);
  EXPECT_NEAR(trim.qty, 10.0, kTol);
  EXPECT_NEAR(book.cash(), 700.0, kTol);
  book.sell_base(make_bar(4, 20.0), 40.0, TestRule::trim);
  EXPECT_TRUE(book.flat());
}

namespace {
void below_venue_minimum() {
  Position book{Quote::require(100.0)};
  book.buy_notional(make_bar(1, 10.0), 0.0, 8.0, TestRule::entry);
}
void oversell() {
  Position book{Quote::require(100.0)};
  book.buy_notional(make_bar(1, 10.0), 50.0, 8.0, TestRule::entry);
  book.sell_base(make_bar(2, 10.0), 6.0, TestRule::trim);
}
void overspend() {
  Position book{Quote::require(100.0)};
  book.buy_notional(make_bar(1, 10.0), 150.0, 8.0, TestRule::entry);
}
} // namespace

TEST(SizingPosition, MisuseExits) {
  EXPECT_EXIT(below_venue_minimum(), ::testing::ExitedWithCode(1),
              "buy_notional invalid");
  EXPECT_EXIT(overspend(), ::testing::ExitedWithCode(1),
              "buy_notional invalid");
  EXPECT_EXIT(oversell(), ::testing::ExitedWithCode(1), "sell_base invalid");
}

} // namespace dagobert::sizing
