#include "sweep.hpp"

#include "exit_policy.hpp"
#include "report.hpp"

#include "tradingcopilot/v1/run_report.pb.h"
#include "tradingcopilot/v1/sweep.pb.h"

#include <google/protobuf/io/zero_copy_stream_impl.h>
#include <google/protobuf/util/delimited_message_util.h>

#include <iostream>
#include <sstream>
#include <vector>

namespace dagobert {
namespace {

[[nodiscard]] std::vector<Bar>
bars_of(const tradingcopilot::v1::SweepRequest &request) {
  std::vector<Bar> bars;
  bars.reserve(static_cast<std::size_t>(request.bars_size()));
  for (const auto &bar : request.bars()) {
    bars.push_back({.open_ms = bar.open_ms(),
                    .open = bar.open(),
                    .high = bar.high(),
                    .low = bar.low(),
                    .close = bar.close(),
                    .volume = bar.volume(),
                    .duration_ms = bar.duration_ms()});
  }
  return bars;
}

[[nodiscard]] tradingcopilot::v1::RunReport
run_item(const tradingcopilot::v1::SweepRequest &request,
         const tradingcopilot::v1::SweepItem &item,
         const std::vector<Bar> &bars, const SweepFactory &factory) {
  tradingcopilot::v1::RunReport report;
  report.set_key(item.key());
  // The strategy constructors report config errors on stderr; capture that
  // text as the item's error instead of the process stderr.
  std::ostringstream captured;
  std::streambuf *const saved = std::cerr.rdbuf(captured.rdbuf());
  try {
    tradingcopilot::v1::RunEnvelope envelope = request.base();
    *envelope.mutable_payload() = item.payload();
    SweepStrategy built = factory(envelope, bars.front().duration_ms);
    built.strategy->set_trade_from(request.trade_from_ms());
    const StrategyResult result = built.strategy->calc_backtest(bars);
    const std::int64_t span_ms = envelope.to_ms() > envelope.from_ms()
                                     ? envelope.to_ms() - envelope.from_ms()
                                     : bars.back().open_ms +
                                           bars.back().duration_ms -
                                           bars.front().open_ms;
    fill_report(result, envelope, span_ms, built.sized, false, report);
  } catch (const std::exception &error) {
    std::string text = captured.str();
    while (!text.empty() && text.back() == '\n') {
      text.pop_back();
    }
    report.Clear();
    report.set_key(item.key());
    report.set_error(text.empty() ? error.what() : text);
  }
  std::cerr.rdbuf(saved);
  return report;
}

} // namespace

int run_sweep(std::istream &in, std::ostream &out,
              const SweepFactory &factory) {
  tradingcopilot::v1::SweepRequest request;
  if (!request.ParseFromIstream(&in)) {
    std::cerr << "stdin is not a SweepRequest\n";
    return 2;
  }
  const std::vector<Bar> bars = bars_of(request);
  if (bars.empty()) {
    std::cerr << "sweep has no bars\n";
    return 1;
  }
  throw_on_config_error() = true;
  {
    google::protobuf::io::OstreamOutputStream stream(&out);
    for (const auto &item : request.items()) {
      const auto report = run_item(request, item, bars, factory);
      if (!google::protobuf::util::SerializeDelimitedToZeroCopyStream(
              report, &stream)) {
        std::cerr << "cannot write report\n";
        return 1;
      }
    }
  } // the stream flushes its buffer into `out` on destruction
  out.flush();
  return 0;
}

} // namespace dagobert
