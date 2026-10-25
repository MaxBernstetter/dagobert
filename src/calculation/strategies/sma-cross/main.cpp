#include "bars_parquet.hpp"
#include "sma_cross_strategy.hpp"

#include "tradingcopilot/v1/run_report.pb.h"

#include <google/protobuf/util/json_util.h>
#include <magic_enum/magic_enum.hpp>

#include <cstdlib>
#include <fstream>
#include <iostream>
#include <iterator>
#include <string>

namespace {

constexpr int kExitRunFailed = 1;
constexpr int kExitUsage = 2;

[[noreturn]] void fail(int code, const std::string &message) {
  std::cerr << message << '\n';
  std::exit(code);
}

[[nodiscard]] std::string arg_value(int argc, char **argv,
                                    const std::string &name) {
  for (int i = 1; i + 1 < argc; ++i) {
    if (name == argv[i]) {
      return argv[i + 1];
    }
  }
  fail(kExitUsage,
       "usage: strategy-sma-cross --config <file> --data-dir <dir>");
}

[[nodiscard]] tradingcopilot::v1::RunEnvelope
read_envelope(const std::string &path) {
  std::ifstream in(path);
  if (!in) {
    fail(kExitUsage, "cannot open config: " + path);
  }
  const std::string json{std::istreambuf_iterator<char>(in), {}};
  tradingcopilot::v1::RunEnvelope envelope;
  const auto status =
      google::protobuf::util::JsonStringToMessage(json, &envelope);
  if (!status.ok()) {
    fail(kExitUsage,
         "config is not a RunEnvelope: " + std::string(status.message()));
  }
  return envelope;
}

void fill_report(const dagobert::StrategyResult &result,
                 const tradingcopilot::v1::RunEnvelope &envelope,
                 std::int64_t duration_ms,
                 tradingcopilot::v1::RunReport &report) {
  report.set_schema_version(envelope.schema_version());
  report.set_label(envelope.label());
  report.mutable_metrics()->set_duration_ms(duration_ms);
  for (const auto &action : result.actions) {
    auto *out = report.add_actions();
    out->set_symbol(envelope.base() + envelope.quote());
    out->set_bar_open_ms(action.bar_open_ms);
    out->set_side(std::string(magic_enum::enum_name(action.side)));
    out->set_weight_pct(action.weight_pct);
    out->set_reason(action.reason);
  }
  for (const auto &fill : result.fills) {
    auto *out = report.add_fills();
    out->set_symbol(envelope.base() + envelope.quote());
    out->set_ts_ms(fill.ts_ms);
    out->set_side(fill.side);
    out->set_qty(fill.qty);
    out->set_price(fill.price);
    out->set_fee(0.0);
  }
}

} // namespace

int main(int argc, char **argv) {
  const std::string config_path = arg_value(argc, argv, "--config");
  const std::string data_dir = arg_value(argc, argv, "--data-dir");
  const tradingcopilot::v1::RunEnvelope envelope = read_envelope(config_path);

  std::vector<dagobert::Bar> bars;
  try {
    bars = dagobert::load_bars_from_lake(data_dir, envelope.provider(),
                                         envelope.base(), envelope.quote(),
                                         envelope.from_ms(), envelope.to_ms());
  } catch (const std::exception &error) {
    fail(kExitRunFailed, std::string("bar load failed: ") + error.what());
  }
  if (bars.empty()) {
    fail(kExitRunFailed, "no bars in range");
  }

  dagobert::SmaCrossStrategy strategy(
      envelope, bars.front().duration_ms,
      dagobert::SmaCrossStrategy::sizing_of(envelope));
  const dagobert::StrategyResult result = strategy.calc_backtest(bars);

  tradingcopilot::v1::RunReport report;
  fill_report(result, envelope, envelope.to_ms() - envelope.from_ms(), report);
  if (!report.SerializeToOstream(&std::cout)) {
    fail(kExitRunFailed, "cannot write report");
  }
  std::cout.flush();
  return 0;
}
