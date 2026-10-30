#include "tradingcopilot/v1/run_envelope.pb.h"
#include "sma_cross_strategy.hpp"

#include "position.hpp"
#include "series.hpp"
#include "sizing_proto.hpp"

#include "tradingcopilot/v1/strategy.pb.h"

#include "exit_policy.hpp"

#include <iostream>

namespace dagobert {
namespace {

// ATR window of the sized flow, in hours (matches the Go ma-cross playbook).
constexpr std::int32_t kAtrWindowHr = 24;

constexpr const char *kMaCrossLabel = "sma-cross";
constexpr const char *kMaCrossTypeUrl =
    "type.googleapis.com/tradingcopilot.v1.MaCrossConfig";

[[nodiscard]] tradingcopilot::v1::MaCrossConfig
unpack_ma_cross(const tradingcopilot::v1::RunEnvelope &envelope) {
  if (envelope.label() != kMaCrossLabel) {
    std::cerr << "label must be sma-cross: " << envelope.label() << '\n';
    exit_config();
  }
  if (envelope.payload().type_url() != kMaCrossTypeUrl) {
    std::cerr << "payload type_url must be " << kMaCrossTypeUrl << ": "
              << envelope.payload().type_url() << '\n';
    exit_config();
  }
  tradingcopilot::v1::MaCrossConfig config;
  if (!envelope.payload().UnpackTo(&config)) {
    std::cerr << "payload must unpack to MaCrossConfig\n";
    exit_config();
  }
  return config;
}

} // namespace

SmaCrossStrategy::SmaCrossStrategy(
    const tradingcopilot::v1::RunEnvelope &envelope,
    std::int64_t bar_duration_ms, std::optional<sizing::SizingConfig> sizing)
    : Strategy(envelope),
      fast_(bars_for(unpack_ma_cross(envelope).fast_hr(), bar_duration_ms)),
      slow_(bars_for(unpack_ma_cross(envelope).slow_hr(), bar_duration_ms)),
      atr_(sizing ? bars_for(kAtrWindowHr, bar_duration_ms) : 1U),
      sizing_(sizing),
      cross_(sizing ? SmaCrossSignal{*sizing} : SmaCrossSignal{}),
      book_(initial_quote()) {}

std::optional<sizing::SizingConfig>
SmaCrossStrategy::sizing_of(const tradingcopilot::v1::RunEnvelope &envelope) {
  const auto config = unpack_ma_cross(envelope);
  if (!config.has_sizing()) {
    return std::nullopt;
  }
  return sizing_from_proto(config.sizing());
}

StrategyResult SmaCrossStrategy::calc_backtest(std::span<const Bar> bars) {
  for (const Bar &bar : bars) {
    require_finite_close(bar);
  }
  if (sizing_) {
    // Fresh indicators and book: the sized flow is stateful per bar.
    SMA fast{fast_.period()};
    SMA slow{slow_.period()};
    ATR atr{atr_.period()};
    SmaCrossSignal cross{*sizing_};
    Position account{initial_quote()};
    for (const Bar &bar : bars) {
      const auto fast_mean = fast.push(bar.close);
      const auto slow_mean = slow.push(bar.close);
      const auto atr_value = atr.push(bar.high, bar.low, bar.close);
      if (bar.open_ms < trade_from_ms()) {
        cross.warm(fast_mean, slow_mean);
        continue;
      }
      cross.step_sized(account, bar, fast_mean, slow_mean, atr_value);
    }
    return strategy_result(account, bars.empty() ? 0.0 : bars.back().close);
  }
  const CloseSeries close = closes_of(bars);
  const CloseSeries fast_mean = fast_.apply_indicator(close);
  const CloseSeries slow_mean = slow_.apply_indicator(close);
  const SmaCrossSeries mask = SmaCrossSignal::evaluate(fast_mean, slow_mean);

  Position account{initial_quote()};
  (void)SmaCrossSignal::actions_from(account, bars, mask, trade_from_ms());
  const double mark = bars.empty() ? 0.0 : bars.back().close;
  return strategy_result(account, mark);
}

StrategyResult SmaCrossStrategy::calc_live(const Bar &bar) {
  require_finite_close(bar);
  if (sizing_) {
    cross_.step_sized(book_, bar, fast_.push(bar.close), slow_.push(bar.close),
                      atr_.push(bar.high, bar.low, bar.close));
    return strategy_result(book_, bar.close);
  }
  if (const auto rule = cross_.apply(fast_.push(bar.close),
                                     slow_.push(bar.close), book_.flat())) {
    cross_.execute(book_, bar, *rule);
  }
  return strategy_result(book_, bar.close);
}

void SmaCrossStrategy::reset() {
  fast_.reset();
  slow_.reset();
  atr_.reset();
  cross_.reset();
  book_ = Position{initial_quote()};
}

} // namespace dagobert
