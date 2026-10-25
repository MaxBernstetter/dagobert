#include "consensus_strategy.hpp"

#include "series.hpp"

#include "tradingcopilot/v1/strategy.pb.h"

#include <cstdlib>
#include <iostream>

namespace dagobert {
namespace {

constexpr const char *kLabel = "signal-consensus";
constexpr const char *kTypeUrl =
    "type.googleapis.com/tradingcopilot.v1.SignalConsensusConfig";

// Hours baked into the Go playbook; RSI and the MACD periods are in bars.
constexpr std::int32_t kSignalHr = 12;
constexpr std::int32_t kRegimeHr = 24;
constexpr std::int32_t kMacroHr = 48;
constexpr std::int32_t kAtrHr = 24;
constexpr std::int32_t kVolumeMeanHr = 24;
constexpr std::size_t kRsiPeriod = 14;
constexpr std::size_t kMacdFast = 12;
constexpr std::size_t kMacdSlow = 26;
constexpr std::size_t kMacdSignal = 9;

// Validates label and payload type. The thresholds are the compiled gate
// constants; the payload is accepted but not read yet.
void check_payload(const tradingcopilot::v1::RunEnvelope &envelope) {
  if (envelope.label() != kLabel) {
    std::cerr << "label must be signal-consensus: " << envelope.label() << '\n';
    std::exit(2);
  }
  if (envelope.payload().type_url() != kTypeUrl) {
    std::cerr << "payload type_url must be " << kTypeUrl << ": "
              << envelope.payload().type_url() << '\n';
    std::exit(2);
  }
  tradingcopilot::v1::SignalConsensusConfig config;
  if (!envelope.payload().UnpackTo(&config)) {
    std::cerr << "payload must unpack to SignalConsensusConfig\n";
    std::exit(2);
  }
}

} // namespace

ConsensusStrategy::ConsensusStrategy(
    const tradingcopilot::v1::RunEnvelope &envelope,
    std::int64_t bar_duration_ms)
    : Strategy(envelope), signal_(bars_for(kSignalHr, bar_duration_ms)),
      regime_(bars_for(kRegimeHr, bar_duration_ms)),
      macro_(bars_for(kMacroHr, bar_duration_ms)), rsi_(kRsiPeriod),
      vol_mean_(bars_for(kVolumeMeanHr, bar_duration_ms)),
      atr_(bars_for(kAtrHr, bar_duration_ms)),
      macd_(kMacdFast, kMacdSlow, kMacdSignal), book_(initial_quote()) {
  check_payload(envelope);
}

StrategyResult ConsensusStrategy::calc_backtest(std::span<const Bar> bars) {
  for (const Bar &bar : bars) {
    require_finite_close(bar);
  }
  const CloseSeries close = closes_of(bars);
  const VolumeSeries volume = volumes_of(bars);
  std::vector<double> high;
  std::vector<double> low;
  for (const Bar &bar : bars) {
    high.push_back(bar.high);
    low.push_back(bar.low);
  }
  const Column signal = signal_.apply_indicator(close);
  const Column regime = regime_.apply_indicator(close);
  const Column macro = macro_.apply_indicator(close);
  const Column rsi = rsi_.apply_indicator(close);
  const Column vol_mean = vol_mean_.apply_indicator(volume);
  const Column atr = atr_.apply_indicator(high, low, close);
  const MacdColumns macd = macd_.apply_indicator(close);
  const ConsensusSeries mask =
      ConsensusSignal::evaluate({close, volume, signal, regime, macro, rsi,
                                 vol_mean, atr, macd.histogram});

  Position account{initial_quote()};
  (void)ConsensusSignal::actions_from(account, bars, mask);
  const double mark = bars.empty() ? 0.0 : bars.back().close;
  return strategy_result(account, mark);
}

StrategyResult ConsensusStrategy::calc_live(const Bar &bar) {
  require_finite_close(bar);
  const ConsensusSample sample{bar.close,
                               bar.volume,
                               signal_.push(bar.close),
                               regime_.push(bar.close),
                               macro_.push(bar.close),
                               rsi_.push(bar.close),
                               vol_mean_.push(bar.volume),
                               atr_.push(bar.high, bar.low, bar.close),
                               macd_.push(bar.close).histogram};
  if (const auto rule = consensus_.apply(sample, book_.flat())) {
    consensus_.execute(book_, bar, *rule);
  }
  return strategy_result(book_, bar.close);
}

void ConsensusStrategy::reset() {
  signal_.reset();
  regime_.reset();
  macro_.reset();
  rsi_.reset();
  vol_mean_.reset();
  atr_.reset();
  macd_.reset();
  consensus_.reset();
  book_ = Position{initial_quote()};
}

} // namespace dagobert
