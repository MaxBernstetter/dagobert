// System tests: start the compiled strategy-signal-consensus binary and check
// its exit code and stdout.
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
    fs::path(__FILE__).parent_path() / "fixtures" / "lake_consensus";

struct BinaryRun {
  int exit_code{-1};
  std::string out;
};

std::string config_json(const std::string &label, double quote,
                        int schema_version = 2,
                        const std::string &type = "SignalConsensusConfig",
                        const std::string &sizing = "") {
  return R"({"schemaVersion":)" + std::to_string(schema_version) +
         R"(,"label":")" + label +
         R"(","provider":"demo","base":"BTC","quote":"USDT",)"
         R"("fromMs":"1704067200000","toMs":"1704844800000","initialQuote":)" +
         std::to_string(quote) +
         R"(,"payload":{"@type":"type.googleapis.com/tradingcopilot.v1.)" +
         type + R"(")" + (sizing.empty() ? "" : R"(,"sizing":)" + sizing) +
         "}}";
}

BinaryRun run_binary(const std::string &config) {
  const fs::path path = fs::temp_directory_path() / "consensus_exe_cfg.json";
  std::ofstream(path) << config;
  const std::string cmd = std::string(CONSENSUS_BIN) + " --config " +
                          path.string() + " --data-dir " + kLake.string() +
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

TEST(ConsensusExe, BuysAndHitsBothSellRules) {
  const BinaryRun run = run_binary(config_json("signal-consensus", 1000.0));
  ASSERT_EQ(run.exit_code, 0);
  tradingcopilot::v1::RunReport report;
  ASSERT_TRUE(report.ParseFromString(run.out));
  EXPECT_EQ(report.label(), "signal-consensus");
  ASSERT_GE(report.fills_size(), 4);
  EXPECT_EQ(report.fills(0).side(), "buy");
  EXPECT_EQ(report.actions_size(), report.fills_size());
  std::set<std::string> reasons;
  for (const auto &action : report.actions()) {
    reasons.insert(action.reason());
  }
  EXPECT_TRUE(reasons.count("consensus_buy"));
  EXPECT_TRUE(reasons.count("consensus_sell"));
  EXPECT_TRUE(reasons.count("atr_stop"));
}

TEST(ConsensusExe, WrongLabelExits2) {
  const BinaryRun run = run_binary(config_json("sma-cross", 1000.0));
  EXPECT_EQ(run.exit_code, 2);
  EXPECT_TRUE(run.out.empty());
}

TEST(ConsensusExe, WrongPayloadTypeExits2) {
  EXPECT_EQ(
      run_binary(config_json("signal-consensus", 1000.0, 2, "MaCrossConfig"))
          .exit_code,
      2);
}

TEST(ConsensusExe, NonPositiveQuoteExits2) {
  EXPECT_EQ(run_binary(config_json("signal-consensus", 0.0)).exit_code, 2);
}

TEST(ConsensusExe, PreviousSchemaVersionExits2) {
  EXPECT_EQ(run_binary(config_json("signal-consensus", 1000.0, 1)).exit_code,
            2);
}

TEST(ConsensusExe, UnknownSchemaVersionExits2) {
  EXPECT_EQ(run_binary(config_json("signal-consensus", 1000.0, 9)).exit_code,
            2);
}

TEST(ConsensusExe, MalformedConfigExits2) {
  EXPECT_EQ(run_binary("not json").exit_code, 2);
}

TEST(ConsensusExe, EmptyRangeExits1) {
  std::string cfg = config_json("signal-consensus", 1000.0);
  cfg.replace(cfg.find("1704067200000"), 13, "1804067200000");
  cfg.replace(cfg.find("1704844800000"), 13, "1804844800000");
  EXPECT_EQ(run_binary(cfg).exit_code, 1);
}

TEST(ConsensusExe, SizedRunReportsSizedFillsAndRules) {
  const BinaryRun run = run_binary(config_json("signal-consensus", 1000.0, 2,
                                               "SignalConsensusConfig",
                                               R"({"unitRiskPct":0.25})"));
  ASSERT_EQ(run.exit_code, 0);
  tradingcopilot::v1::RunReport report;
  ASSERT_TRUE(report.ParseFromString(run.out));
  ASSERT_GE(report.fills_size(), 2);
  EXPECT_EQ(report.actions_size(), report.fills_size());
  EXPECT_EQ(report.fills(0).side(), "buy");
  EXPECT_LT(report.fills(0).qty() * report.fills(0).price(), 1000.0);
  ASSERT_EQ(report.sizing_size(), report.actions_size());
  for (int i = 0; i < report.sizing_size(); ++i) {
    const auto &sz = report.sizing(i);
    EXPECT_EQ(sz.bar_open_ms(), report.actions(i).bar_open_ms());
    EXPECT_EQ(sz.rule_id(), report.actions(i).reason());
    EXPECT_GT(sz.notional(), 0.0);
  }
  const auto &buy = report.sizing(0);
  EXPECT_NEAR(buy.notional(), report.fills(0).qty() * report.fills(0).price(),
              1e-6);
  EXPECT_LT(buy.stop_price(), report.fills(0).price());
  EXPECT_DOUBLE_EQ(buy.requested_risk_pct(), 0.25);
  EXPECT_GT(buy.capped_risk_pct(), 0.0);
  EXPECT_LE(buy.capped_risk_pct(), buy.requested_risk_pct() + 1e-9);
  std::set<std::string> reasons;
  for (const auto &action : report.actions()) {
    reasons.insert(action.reason());
  }
  EXPECT_TRUE(reasons.count("consensus_buy"));
  EXPECT_TRUE(reasons.count("atr_stop"));
}

TEST(ConsensusExe, InvalidSizingExits2) {
  const BinaryRun run =
      run_binary(config_json("signal-consensus", 1000.0, 2,
                             "SignalConsensusConfig", R"({"unitRiskPct":3})"));
  EXPECT_EQ(run.exit_code, 2);
  EXPECT_TRUE(run.out.empty());
}
