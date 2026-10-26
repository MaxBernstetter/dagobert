#pragma once

#include <cstddef>
#include <cstdint>
#include <string>
#include <vector>

namespace dagobert {

enum class Side { none, buy, sell };

struct Bar {
  std::int64_t open_ms{};
  double open{};
  double high{};
  double low{};
  double close{};
  double volume{};
  std::int64_t duration_ms{};
  // Close of each source candle inside an aggregated bar, in time order (see
  // aggregate_bars). Empty for bars read as they are stored.
  std::vector<double> segment_close;
};

struct Column {
  std::vector<double> samples;
  std::vector<char> ready;
};

struct CloseSeries : Column {};
struct VolumeSeries : Column {};

struct Action {
  std::int64_t bar_open_ms{};
  Side side{Side::none};
  std::string reason;
  double weight_pct{};
  double stop{};
  double notional{};
  double requested_risk_pct{};
  double capped_risk_pct{};
  // Base quantity traded, equity at the decision bar close, and the step by
  // which a pyramid raised the earlier stops.
  double base_qty{};
  double equity{};
  double raise_stops_by{};
  // vol_cut: heat budget in base per unit of equity (see the proto).
  double heat_base_per_equity{};
  // Buys: the strategy's notional before the cash clamp (notional is what was spent).
  double requested_notional{};
};

struct Fill {
  std::int64_t ts_ms{};
  std::string side;
  double qty{};
  double price{};
  std::string reason;
};

struct StrategyResult {
  double cash{};
  double qty{};
  double equity{};
  std::vector<Action> actions;
  std::vector<Fill> fills;
};

} // namespace dagobert
