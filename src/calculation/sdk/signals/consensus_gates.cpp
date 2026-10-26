#include "consensus_gates.hpp"

#include <cstddef>

namespace dagobert {
namespace {

[[nodiscard]] std::optional<double> at(const Column &col, std::size_t i) {
  if (i >= col.samples.size() || i >= col.ready.size() || !col.ready[i]) {
    return std::nullopt;
  }
  return col.samples[i];
}

} // namespace

std::vector<Cross> BaselineCross::evaluate(const CloseSeries &close,
                                           const Column &baseline) {
  BaselineCross gate;
  std::vector<Cross> out(close.samples.size(), Cross::none);
  for (std::size_t i = 0; i < out.size(); ++i) {
    if (close.ready[i]) {
      out[i] = gate.apply(close.samples[i], at(baseline, i));
    }
  }
  return out;
}

Cross BaselineCross::apply(double close, std::optional<double> baseline) {
  Cross out = Cross::none;
  if (baseline && prev_close_ && prev_baseline_) {
    if (*prev_close_ <= *prev_baseline_ && close > *baseline) {
      out = Cross::up;
    } else if (*prev_close_ >= *prev_baseline_ && close < *baseline) {
      out = Cross::down;
    }
  }
  prev_close_ = close;
  prev_baseline_ = baseline;
  return out;
}

void BaselineCross::reset() {
  prev_close_.reset();
  prev_baseline_.reset();
}

std::vector<char> HigherTrend::evaluate(const CloseSeries &close,
                                        const Column &regime,
                                        const Column &macro) {
  std::vector<char> out(close.samples.size(), 0);
  for (std::size_t i = 0; i < out.size(); ++i) {
    out[i] =
        close.ready[i] && apply(close.samples[i], at(regime, i), at(macro, i));
  }
  return out;
}

bool HigherTrend::apply(double close, std::optional<double> regime,
                        std::optional<double> macro) {
  if (!regime || !macro || *regime <= 0.0 || *macro <= 0.0) {
    return false;
  }
  return close > *regime && close > *macro && *regime > *macro;
}

std::vector<Vote> RsiVote::evaluate(const Column &rsi,
                                    const ConsensusParams &params) {
  RsiVote gate{params};
  std::vector<Vote> out(rsi.samples.size(), Vote::neutral);
  for (std::size_t i = 0; i < out.size(); ++i) {
    out[i] = gate.apply(at(rsi, i));
  }
  return out;
}

Vote RsiVote::apply(std::optional<double> rsi) {
  Vote out = Vote::neutral;
  if (rsi) {
    if (*rsi >= params_.rsi_bull || (*rsi > 50.0 && prev_ && *rsi > *prev_)) {
      out = Vote::bull;
    } else if (*rsi <= params_.rsi_bear ||
               (*rsi < 50.0 && prev_ && *rsi < *prev_)) {
      out = Vote::bear;
    }
  }
  prev_ = rsi;
  return out;
}

void RsiVote::reset() { prev_.reset(); }

std::vector<Vote> VolumeRatio::evaluate(const VolumeSeries &volume,
                                        const Column &mean,
                                        const ConsensusParams &params) {
  std::vector<Vote> out(volume.samples.size(), Vote::neutral);
  for (std::size_t i = 0; i < out.size(); ++i) {
    if (volume.ready[i]) {
      out[i] = apply(volume.samples[i], at(mean, i), params);
    }
  }
  return out;
}

Vote VolumeRatio::apply(double volume, std::optional<double> mean,
                        const ConsensusParams &params) {
  if (!mean || volume <= 0.0 || *mean <= 0.0) {
    return Vote::neutral;
  }
  const double ratio = volume / *mean;
  if (ratio >= params.vol_ratio_bull) {
    return Vote::bull;
  }
  return ratio <= params.vol_ratio_bear ? Vote::bear : Vote::neutral;
}

std::vector<char> MacdSign::evaluate(const Column &histogram,
                                     const std::vector<Vote> &momentum) {
  std::vector<char> out(histogram.samples.size(), 0);
  for (std::size_t i = 0; i < out.size() && i < momentum.size(); ++i) {
    out[i] = apply(at(histogram, i), momentum[i]);
  }
  return out;
}

bool MacdSign::apply(std::optional<double> histogram, Vote momentum) {
  return histogram ? *histogram > 0.0 : momentum == Vote::bull;
}

std::vector<char> AtrStop::evaluate(const CloseSeries &close,
                                    const Column &baseline, const Column &atr,
                                    const ConsensusParams &params) {
  std::vector<char> out(close.samples.size(), 0);
  for (std::size_t i = 0; i < out.size(); ++i) {
    out[i] = close.ready[i] &&
             apply(close.samples[i], at(baseline, i), at(atr, i), params);
  }
  return out;
}

bool AtrStop::apply(double close, std::optional<double> baseline,
                    std::optional<double> atr, const ConsensusParams &params) {
  return baseline && atr && close < *baseline - params.atr_stop_mult * *atr;
}

} // namespace dagobert
