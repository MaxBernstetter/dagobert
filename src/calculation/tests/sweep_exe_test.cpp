// System tests of `--sweep`: a sweep over shared bars must equal separate
// single runs of the same executable.
#include <gtest/gtest.h>

#include "bars_parquet.hpp"

#include "tradingcopilot/v1/run_report.pb.h"
#include "tradingcopilot/v1/sweep.pb.h"

#include <google/protobuf/io/zero_copy_stream_impl_lite.h>
#include <google/protobuf/util/delimited_message_util.h>
#include <google/protobuf/util/json_util.h>

#include <sys/wait.h>

#include <cstdio>
#include <filesystem>
#include <fstream>
#include <string>
#include <vector>

namespace {

namespace fs = std::filesystem;
using tradingcopilot::v1::RunEnvelope;
using tradingcopilot::v1::RunReport;
using tradingcopilot::v1::SweepRequest;

const fs::path kLake =
    fs::path(__FILE__).parent_path() / "fixtures" / "lake_consensus";
constexpr std::int64_t kFrom = 1704067200000;
constexpr std::int64_t kTo = 1704844800000;

struct Strategy {
  std::string bin;
  std::string label;
  std::string type;
  std::string body; // config fields before the optional sizing block
};

const Strategy kSmaCross{SMA_CROSS_BIN, "sma-cross", "MaCrossConfig",
                         R"("fastHr":3,"slowHr":6)"};
const Strategy kConsensus{CONSENSUS_BIN, "signal-consensus",
                          "SignalConsensusConfig", R"("rsiBull":60)"};

std::string envelope_json(const Strategy &s, const std::string &sizing,
                          const std::string &type_override = "") {
  return R"({"schemaVersion":2,"label":")" + s.label +
         R"(","provider":"demo","base":"BTC","quote":"USDT","fromMs":")" +
         std::to_string(kFrom) + R"(","toMs":")" + std::to_string(kTo) +
         R"(","initialQuote":1000,"payload":{"@type":"type.googleapis.com/tradingcopilot.v1.)" +
         (type_override.empty() ? s.type : type_override) + R"(")" +
         (type_override.empty() ? "," + s.body : "") +
         (sizing.empty() ? "" : R"(,"sizing":)" + sizing) + "}}";
}

RunEnvelope parse(const std::string &json) {
  RunEnvelope envelope;
  EXPECT_TRUE(
      google::protobuf::util::JsonStringToMessage(json, &envelope).ok());
  return envelope;
}

std::string run_shell(const std::string &cmd, int &exit_code) {
  std::string out;
  FILE *pipe = popen(cmd.c_str(), "r");
  EXPECT_NE(pipe, nullptr);
  char buf[4096];
  std::size_t n = 0;
  while ((n = fread(buf, 1, sizeof buf, pipe)) > 0) {
    out.append(buf, n);
  }
  exit_code = WEXITSTATUS(pclose(pipe));
  return out;
}

RunReport single_run(const Strategy &s, const std::string &sizing) {
  const fs::path cfg = fs::temp_directory_path() / "sweep_single_cfg.json";
  std::ofstream(cfg) << envelope_json(s, sizing);
  int code = -1;
  const std::string out =
      run_shell(s.bin + " --config " + cfg.string() + " --data-dir " +
                    kLake.string() + " 2>/dev/null",
                code);
  EXPECT_EQ(code, 0);
  RunReport report;
  EXPECT_TRUE(report.ParseFromString(out));
  return report;
}

SweepRequest request_of(const Strategy &s,
                        const std::vector<std::string> &sizings,
                        std::int64_t trade_from_ms) {
  SweepRequest request;
  *request.mutable_base() = parse(envelope_json(s, ""));
  request.mutable_base()->clear_payload();
  for (const auto &bar : dagobert::load_bars_from_lake(
           kLake.string(), "demo", "BTC", "USDT", kFrom, kTo)) {
    auto *out = request.add_bars();
    out->set_open_ms(bar.open_ms);
    out->set_open(bar.open);
    out->set_high(bar.high);
    out->set_low(bar.low);
    out->set_close(bar.close);
    out->set_volume(bar.volume);
    out->set_duration_ms(bar.duration_ms);
  }
  request.set_trade_from_ms(trade_from_ms);
  int index = 0;
  for (const auto &sizing : sizings) {
    auto *item = request.add_items();
    item->set_key("item-" + std::to_string(index++));
    *item->mutable_payload() = parse(envelope_json(s, sizing)).payload();
  }
  return request;
}

std::vector<RunReport> sweep_run(const Strategy &s, const SweepRequest &request,
                                 int &exit_code) {
  const fs::path in = fs::temp_directory_path() / "sweep_request.bin";
  std::ofstream(in, std::ios::binary) << request.SerializeAsString();
  const std::string out = run_shell(
      s.bin + " --sweep < " + in.string() + " 2>/dev/null", exit_code);
  std::vector<RunReport> reports;
  google::protobuf::io::ArrayInputStream stream(out.data(),
                                                static_cast<int>(out.size()));
  bool clean_eof = false;
  for (;;) {
    RunReport report;
    if (!google::protobuf::util::ParseDelimitedFromZeroCopyStream(
            &report, &stream, &clean_eof)) {
      break;
    }
    reports.push_back(report);
  }
  EXPECT_TRUE(clean_eof);
  return reports;
}

void expect_same_decisions(const RunReport &sweep, const RunReport &single) {
  ASSERT_EQ(sweep.actions_size(), single.actions_size());
  for (int i = 0; i < sweep.actions_size(); ++i) {
    EXPECT_EQ(sweep.actions(i).SerializeAsString(),
              single.actions(i).SerializeAsString());
  }
  ASSERT_EQ(sweep.sizing_size(), single.sizing_size());
  for (int i = 0; i < sweep.sizing_size(); ++i) {
    EXPECT_EQ(sweep.sizing(i).SerializeAsString(),
              single.sizing(i).SerializeAsString());
  }
  EXPECT_EQ(sweep.fills_size(), 0);
}

void check_three_configs(const Strategy &s,
                         const std::vector<std::string> &sizings) {
  int code = -1;
  const auto reports = sweep_run(s, request_of(s, sizings, kFrom), code);
  ASSERT_EQ(code, 0);
  ASSERT_EQ(reports.size(), sizings.size());
  bool any_sized_fields = false;
  for (std::size_t i = 0; i < sizings.size(); ++i) {
    EXPECT_EQ(reports[i].key(), "item-" + std::to_string(i));
    EXPECT_TRUE(reports[i].error().empty());
    const RunReport single = single_run(s, sizings[i]);
    expect_same_decisions(reports[i], single);
    EXPECT_GT(single.fills_size(), 0);
    for (const auto &action : reports[i].actions()) {
      EXPECT_EQ(action.has_equity(), !sizings[i].empty());
      any_sized_fields |=
          action.has_quote_notional() && action.has_stop_price();
    }
  }
  EXPECT_TRUE(any_sized_fields);
}

void check_invalid_item(const Strategy &s) {
  SweepRequest request =
      request_of(s, {"", R"({"unitRiskPct":99})", ""}, kFrom);
  // Item 2 carries a payload of the wrong type.
  *request.mutable_items(2)->mutable_payload() =
      parse(envelope_json(s, "",
                          s.type == "MaCrossConfig" ? "SignalConsensusConfig"
                                                    : "MaCrossConfig"))
          .payload();
  int code = -1;
  const auto reports = sweep_run(s, request, code);
  ASSERT_EQ(code, 0);
  ASSERT_EQ(reports.size(), 3U);
  EXPECT_TRUE(reports[0].error().empty());
  EXPECT_GT(reports[0].actions_size(), 0);
  EXPECT_EQ(reports[1].key(), "item-1");
  EXPECT_FALSE(reports[1].error().empty());
  EXPECT_EQ(reports[1].actions_size(), 0);
  EXPECT_EQ(reports[2].key(), "item-2");
  EXPECT_FALSE(reports[2].error().empty());
}

void check_warmup(const Strategy &s, const std::string &sizing) {
  int code = -1;
  const auto all = sweep_run(s, request_of(s, {sizing}, kFrom), code);
  ASSERT_EQ(all.size(), 1U);
  ASSERT_GT(all[0].actions_size(), 1);
  // Trade only from the middle of the range; the earlier bars warm up.
  const std::int64_t cut = kFrom + (kTo - kFrom) / 2;
  const auto late = sweep_run(s, request_of(s, {sizing}, cut), code);
  ASSERT_EQ(code, 0);
  ASSERT_EQ(late.size(), 1U);
  EXPECT_LT(late[0].actions_size(), all[0].actions_size());
  for (const auto &action : late[0].actions()) {
    EXPECT_GE(action.bar_open_ms(), cut);
  }
  // No trading at all before the first bar's successor range end.
  const auto none = sweep_run(s, request_of(s, {sizing}, kTo), code);
  ASSERT_EQ(none.size(), 1U);
  EXPECT_EQ(none[0].actions_size(), 0);
}

} // namespace

TEST(SweepSmaCross, MatchesSingleRuns) {
  check_three_configs(kSmaCross, {"", R"({"unitRiskPct":0.5})",
                                  R"({"unitRiskPct":0.25,"maxUnits":3})"});
}

TEST(SweepSmaCross, InvalidItemYieldsErrorReport) {
  check_invalid_item(kSmaCross);
}

TEST(SweepSmaCross, WarmUpBarsProduceNoActions) {
  check_warmup(kSmaCross, "");
  check_warmup(kSmaCross, R"({"unitRiskPct":0.5})");
}

TEST(SweepConsensus, MatchesSingleRuns) {
  check_three_configs(kConsensus, {"", R"({"unitRiskPct":0.25})",
                                   R"({"unitRiskPct":0.5,"maxUnits":3})"});
}

TEST(SweepConsensus, InvalidItemYieldsErrorReport) {
  check_invalid_item(kConsensus);
}

TEST(SweepConsensus, WarmUpBarsProduceNoActions) {
  check_warmup(kConsensus, "");
  check_warmup(kConsensus, R"({"unitRiskPct":0.25})");
}

TEST(SweepExe, GarbageStdinExits2) {
  int code = -1;
  const fs::path in = fs::temp_directory_path() / "sweep_garbage.bin";
  std::ofstream(in, std::ios::binary) << "\xff\xff\xff not a proto";
  (void)run_shell(std::string(SMA_CROSS_BIN) + " --sweep < " + in.string() +
                      " 2>/dev/null",
                  code);
  EXPECT_EQ(code, 2);
}
