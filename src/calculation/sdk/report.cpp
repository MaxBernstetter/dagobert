#include "report.hpp"

#include "sizing_proto.hpp"

#include <magic_enum/magic_enum.hpp>

#include <string>

namespace dagobert {
namespace {

void write_action(const Action &action, const std::string &symbol, bool sized,
                  tradingcopilot::v1::ExperimentInstanceAction &out) {
  out.set_symbol(symbol);
  out.set_bar_open_ms(action.bar_open_ms);
  out.set_side(std::string(magic_enum::enum_name(action.side)));
  out.set_weight_pct(action.weight_pct);
  out.set_reason(action.reason);
  if (!sized) {
    return;
  }
  out.set_equity(action.equity);
  if (action.side == Side::buy) {
    out.set_quote_notional(action.notional);
    out.set_stop_price(action.stop);
    if (action.reason == "pyramid") {
      out.set_raise_stops_by(action.raise_stops_by);
    }
  } else if (action.reason == "vol_cut") {
    out.set_base_qty(action.base_qty);
  }
}

} // namespace

void fill_report(const StrategyResult &result,
                 const tradingcopilot::v1::RunEnvelope &envelope,
                 std::int64_t duration_ms, bool sized, bool with_fills,
                 tradingcopilot::v1::RunReport &report) {
  const std::string symbol = envelope.base() + envelope.quote();
  report.set_schema_version(envelope.schema_version());
  report.set_label(envelope.label());
  report.mutable_metrics()->set_duration_ms(duration_ms);
  for (const Action &action : result.actions) {
    write_action(action, symbol, sized, *report.add_actions());
  }
  if (with_fills) {
    for (const Fill &fill : result.fills) {
      auto *out = report.add_fills();
      out->set_symbol(symbol);
      out->set_ts_ms(fill.ts_ms);
      out->set_side(fill.side);
      out->set_qty(fill.qty);
      out->set_price(fill.price);
      out->set_fee(0.0);
    }
  }
  if (sized) {
    write_sizing_output(result.actions, report);
  }
}

} // namespace dagobert
