// System tests: start the compiled strategy-sma-cross binary and check its exit
// code and stdout.
#include <gtest/gtest.h>

#include "tradingcopilot/v1/run_report.pb.h"

#include <sys/wait.h>

#include <cstdio>
#include <filesystem>
#include <fstream>
#include <set>
#include <string>

namespace {

namespace fs = std::filesystem;

const fs::path kLake =
    fs::path(__FILE__).parent_path() / "fixtures" / "lake_roundtrip";

struct BinaryRun {
  int exit_code{-1};
  std::string out;
};

std::string config_json(const std::string &label, double quote,
                        int schema_version = 1,
                        const std::string &sizing = "") {
  return R"({"schemaVersion":)" + std::to_string(schema_version) +
         R"(,"label":")" + label +
         R"(","provider":"demo","base":"BTC","quote":"USDT",)"
         R"("fromMs":"1704067200000","toMs":"1704153600000","initialQuote":)" +
         std::to_string(quote) +
         R"(,"payload":{"@type":"type.googleapis.com/tradingcopilot.v1.MaCrossConfig",)"
         R"("fastHr":3,"slowHr":6)" +
         (sizing.empty() ? "" : R"(,"sizing":)" + sizing) + "}}";
}

// The 8-day consensus lake has enough bars for the 24-hour ATR window.
const fs::path kLongLake =
    fs::path(__FILE__).parent_path() / "fixtures" / "lake_consensus";
constexpr const char *kLongRange = R"("toMs":"1704844800000")";

BinaryRun run_binary(const std::string &config, bool write_config = true,
                     const fs::path &lake = kLake) {
  const fs::path path = fs::temp_directory_path() / "sma_cross_exe_cfg.json";
  if (write_config) {
    std::ofstream(path) << config;
  }
  const std::string cmd = std::string(SMA_CROSS_BIN) + " --config " +
                          path.string() + " --data-dir " + lake.string() +
                          " 2>/dev/null";
  BinaryRun run;
  FILE *pipe = popen(cmd.c_str(), "r");
  EXPECT_NE(pipe, nullptr);
  char buf[4096];
  std::size_t n = 0;
  while ((n = fread(buf, 1, sizeof buf, pipe)) > 0) {
    run.out.append(buf, n);
  }
  run.exit_code = WEXITSTATUS(pclose(pipe));
  return run;
}

} // namespace

TEST(SmaCrossExe, RoundTripWritesReport) {
  const BinaryRun run = run_binary(config_json("sma-cross", 1000.0));
  ASSERT_EQ(run.exit_code, 0);
  tradingcopilot::v1::RunReport report;
  ASSERT_TRUE(report.ParseFromString(run.out));
  EXPECT_EQ(report.label(), "sma-cross");
  ASSERT_GE(report.fills_size(), 2);
  EXPECT_EQ(report.fills(0).side(), "buy");
  EXPECT_EQ(report.fills(report.fills_size() - 1).side(), "sell");
  EXPECT_EQ(report.actions_size(), report.fills_size());
}

TEST(SmaCrossExe, WrongLabelExits2) {
  const BinaryRun run = run_binary(config_json("other", 1000.0));
  EXPECT_EQ(run.exit_code, 2);
  EXPECT_TRUE(run.out.empty());
}

TEST(SmaCrossExe, NonPositiveQuoteExits2) {
  EXPECT_EQ(run_binary(config_json("sma-cross", 0.0)).exit_code, 2);
}

TEST(SmaCrossExe, UnknownSchemaVersionExits2) {
  EXPECT_EQ(run_binary(config_json("sma-cross", 1000.0, 9)).exit_code, 2);
}

TEST(SmaCrossExe, MalformedConfigExits2) {
  EXPECT_EQ(run_binary("not json").exit_code, 2);
}

TEST(SmaCrossExe, EmptyRangeExits1) {
  std::string cfg = config_json("sma-cross", 1000.0);
  cfg.replace(cfg.find("1704067200000"), 13, "1804067200000");
  cfg.replace(cfg.find("1704153600000"), 13, "1804153600000");
  EXPECT_EQ(run_binary(cfg).exit_code, 1);
}

std::string long_range(std::string cfg) {
  const std::string from = R"("toMs":"1704153600000")";
  cfg.replace(cfg.find(from), from.size(), kLongRange);
  return cfg;
}

TEST(SmaCrossExe, SizedRunOpensStopsOutAndCloses) {
  const BinaryRun run =
      run_binary(long_range(config_json("sma-cross", 1000.0, 1,
                                        R"({"unitRiskPct":0.25})")),
                 true, kLongLake);
  ASSERT_EQ(run.exit_code, 0);
  tradingcopilot::v1::RunReport report;
  ASSERT_TRUE(report.ParseFromString(run.out));
  ASSERT_GE(report.fills_size(), 2);
  EXPECT_EQ(report.actions_size(), report.fills_size());
  EXPECT_EQ(report.fills(0).side(), "buy");
  // Risk-sized: the first buy spends less than the whole book.
  EXPECT_LT(report.fills(0).qty() * report.fills(0).price(), 1000.0);
  std::set<std::string> reasons;
  for (const auto &action : report.actions()) {
    reasons.insert(action.reason());
  }
  EXPECT_TRUE(reasons.count("atr_stop"));
}

TEST(SmaCrossExe, SizedAndUnsizedRunsDiffer) {
  const BinaryRun plain =
      run_binary(long_range(config_json("sma-cross", 1000.0)), true, kLongLake);
  const BinaryRun sized =
      run_binary(long_range(config_json("sma-cross", 1000.0, 1,
                                        R"({"unitRiskPct":0.25})")),
                 true, kLongLake);
  ASSERT_EQ(plain.exit_code, 0);
  ASSERT_EQ(sized.exit_code, 0);
  EXPECT_NE(plain.out, sized.out);
}

TEST(SmaCrossExe, InvalidSizingExits2) {
  const BinaryRun run =
      run_binary(config_json("sma-cross", 1000.0, 1, R"({"unitRiskPct":3})"));
  EXPECT_EQ(run.exit_code, 2);
  EXPECT_TRUE(run.out.empty());
}
