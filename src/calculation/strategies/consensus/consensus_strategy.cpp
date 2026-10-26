#include "consensus_strategy.hpp"

#include "series.hpp"
#include "sizing_proto.hpp"

#include "tradingcopilot/v1/strategy.pb.h"

#include <cstdlib>
#include <iostream>
#include <string>

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

// Validates label and payload type and returns the config. The thresholds are
// read by params_of and sizing_of.
tradingcopilot::v1::SignalConsensusConfig
check_payload(const tradingcopilot::v1::RunEnvelope &envelope) {
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
  return config;
}

} // namespace

ConsensusStrategy::ConsensusStrategy(
    const tradingcopilot::v1::RunEnvelope &envelope,
    std::int64_t bar_duration_ms, std::optional<sizing::SizingConfig> sizing,
    const ConsensusParams &params)
    : Strategy(envelope), signal_(bars_for(kSignalHr, bar_duration_ms)),
      regime_(bars_for(kRegimeHr, bar_duration_ms)),
      macro_(bars_for(kMacroHr, bar_duration_ms)), rsi_(kRsiPeriod),
      vol_mean_(bars_for(kVolumeMeanHr, bar_duration_ms)),
      atr_(bars_for(kAtrHr, bar_duration_ms)),
      macd_(kMacdFast, kMacdSlow, kMacdSignal), params_(params),
      sizing_(sizing), consensus_(sizing, params), book_(initial_quote()) {
  check_payload(envelope);
}

std::optional<sizing::SizingConfig>
ConsensusStrategy::sizing_of(const tradingcopilot::v1::RunEnvelope &envelope) {
  const auto config = check_payload(envelope);
  if (!config.has_sizing()) {
    return std::nullopt;
  }
  return sizing_from_proto(config.sizing());
}

namespace {

[[noreturn]] void invalid(const std::string &what) {
  std::cerr << "signal-consensus config: " << what << '\n';
  std::exit(2);
}

} // namespace

ConsensusParams
ConsensusStrategy::params_of(const tradingcopilot::v1::RunEnvelope &envelope) {
  const auto config = check_payload(envelope);
  ConsensusParams p;
  // proto3 scalars carry no presence: zero means unset. Zero is invalid for
  // every double except rsi_bear and for no cooldown but 0 itself, so an
  // explicit zero cooldown reads as the default.
  const auto pick = [](double v, double fallback) {
    return v == 0.0 ? fallback : v;
  };
  p.rsi_bull = pick(config.rsi_bull(), p.rsi_bull);
  p.rsi_bear = pick(config.rsi_bear(), p.rsi_bear);
  p.vol_ratio_bull = pick(config.vol_ratio_bull(), p.vol_ratio_bull);
  p.vol_ratio_bear = pick(config.vol_ratio_bear(), p.vol_ratio_bear);
  p.atr_stop_mult = pick(config.atr_stop_mult(), p.atr_stop_mult);
  if (config.min_bars_flat_after_sell() < 0 ||
      config.min_bars_long_before_sell() < 0) {
    invalid("cooldown bars must be >= 0");
  }
  if (config.min_bars_flat_after_sell() > 0) {
    p.min_bars_flat_after_sell =
        static_cast<std::size_t>(config.min_bars_flat_after_sell());
  }
  if (config.min_bars_long_before_sell() > 0) {
    p.min_bars_long_before_sell =
        static_cast<std::size_t>(config.min_bars_long_before_sell());
  }
  // Same ranges as Config.Validate in the Go playbook (NaN fails every test).
  if (!(p.rsi_bull > p.rsi_bear)) {
    invalid("rsiBull must be > rsiBear");
  }
  if (!(p.rsi_bull > 0.0 && p.rsi_bull <= 100.0 && p.rsi_bear >= 0.0 &&
        p.rsi_bear < 100.0)) {
    invalid("RSI thresholds must be in (0,100]");
  }
  if (!(p.vol_ratio_bull > 0.0 && p.vol_ratio_bear > 0.0)) {
    invalid("volume ratios must be positive");
  }
  if (!(p.vol_ratio_bull > p.vol_ratio_bear)) {
    invalid("volRatioBull must be > volRatioBear");
  }
  if (!(p.atr_stop_mult > 0.0)) {
    invalid("atrStopMult must be positive");
  }
  return p;
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
                                 vol_mean, atr, macd.histogram},
                                params_);

  Position account{initial_quote()};
  (void)ConsensusSignal::actions_from(account, bars, mask, sizing_, params_);
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
  const auto rule = sizing_ ? consensus_.apply(sample, book_)
                            : consensus_.apply(sample, book_.flat());
  if (rule) {
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
