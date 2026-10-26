#include "bars_parquet.hpp"
#include "sma_cross_strategy.hpp"

#include "report.hpp"
#include "sweep.hpp"

#include "tradingcopilot/v1/run_report.pb.h"

#include <google/protobuf/util/json_util.h>

#include <cstdlib>
#include <fstream>
#include <iostream>
#include <iterator>
#include <memory>
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

[[nodiscard]] dagobert::SweepStrategy
sweep_strategy(const tradingcopilot::v1::RunEnvelope &envelope,
               std::int64_t duration_ms) {
  const auto sizing = dagobert::SmaCrossStrategy::sizing_of(envelope);
  return {std::make_unique<dagobert::SmaCrossStrategy>(envelope, duration_ms,
                                                       sizing),
          sizing.has_value()};
}

} // namespace

int main(int argc, char **argv) {
  for (int i = 1; i < argc; ++i) {
    if (std::string(argv[i]) == "--sweep") {
      return dagobert::run_sweep(std::cin, std::cout, sweep_strategy);
    }
  }
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

  const auto sizing = dagobert::SmaCrossStrategy::sizing_of(envelope);
  dagobert::SmaCrossStrategy strategy(envelope, bars.front().duration_ms,
                                      sizing);
  const dagobert::StrategyResult result = strategy.calc_backtest(bars);

  tradingcopilot::v1::RunReport report;
  dagobert::fill_report(result, envelope, envelope.to_ms() - envelope.from_ms(),
                        sizing.has_value(), true, report);
  if (!report.SerializeToOstream(&std::cout)) {
    fail(kExitRunFailed, "cannot write report");
  }
  std::cout.flush();
  return 0;
}
