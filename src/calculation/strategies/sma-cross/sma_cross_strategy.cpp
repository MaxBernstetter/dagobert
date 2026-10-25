#include "sma_cross_strategy.hpp"

#include "position.hpp"
#include "series.hpp"

#include "tradingcopilot/v1/strategy.pb.h"

#include <cstdlib>
#include <iostream>

namespace dagobert {
namespace {

constexpr const char *kMaCrossLabel = "sma-cross";
constexpr const char *kMaCrossTypeUrl =
    "type.googleapis.com/tradingcopilot.v1.MaCrossConfig";

[[nodiscard]] tradingcopilot::v1::MaCrossConfig
unpack_ma_cross(const tradingcopilot::v1::RunEnvelope &envelope) {
  if (envelope.label() != kMaCrossLabel) {
    std::cerr << "label must be sma-cross: " << envelope.label() << '\n';
    std::exit(2);
  }
  if (envelope.payload().type_url() != kMaCrossTypeUrl) {
    std::cerr << "payload type_url must be " << kMaCrossTypeUrl << ": "
              << envelope.payload().type_url() << '\n';
    std::exit(2);
  }
  tradingcopilot::v1::MaCrossConfig config;
  if (!envelope.payload().UnpackTo(&config)) {
    std::cerr << "payload must unpack to MaCrossConfig\n";
    std::exit(2);
  }
  return config;
}

} // namespace

SmaCrossStrategy::SmaCrossStrategy(
    const tradingcopilot::v1::RunEnvelope &envelope,
    std::int64_t bar_duration_ms)
    : Strategy(envelope),
      fast_(bars_for(unpack_ma_cross(envelope).fast_hr(), bar_duration_ms)),
      slow_(bars_for(unpack_ma_cross(envelope).slow_hr(), bar_duration_ms)),
      book_(initial_quote()) {}

StrategyResult SmaCrossStrategy::calc_backtest(std::span<const Bar> bars) {
  for (const Bar &bar : bars) {
    require_finite_close(bar);
  }
  const CloseSeries close = closes_of(bars);
  const CloseSeries fast_mean = fast_.apply_indicator(close);
  const CloseSeries slow_mean = slow_.apply_indicator(close);
  const SmaCrossSeries mask = SmaCrossSignal::evaluate(fast_mean, slow_mean);

  Position account{initial_quote()};
  (void)SmaCrossSignal::actions_from(account, bars, mask);
  const double mark = bars.empty() ? 0.0 : bars.back().close;
  return strategy_result(account, mark);
}

StrategyResult SmaCrossStrategy::calc_live(const Bar &bar) {
  require_finite_close(bar);
  if (const auto rule = cross_.apply(fast_.push(bar.close),
                                     slow_.push(bar.close), book_.flat())) {
    cross_.execute(book_, bar, *rule);
  }
  return strategy_result(book_, bar.close);
}

void SmaCrossStrategy::reset() {
  fast_.reset();
  slow_.reset();
  cross_.reset();
  book_ = Position{initial_quote()};
}

} // namespace dagobert
