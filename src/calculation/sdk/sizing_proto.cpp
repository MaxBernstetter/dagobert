#include "sizing_proto.hpp"

#include <magic_enum/magic_enum.hpp>

#include "exit_policy.hpp"

#include <cstdlib>
#include <iostream>

namespace dagobert {

sizing::SizingConfig
sizing_from_proto(const tradingcopilot::v1::Sizing &proto) {
  sizing::SizingConfig cfg;
  if (proto.has_unit_risk_pct()) {
    cfg.unit_risk_pct = proto.unit_risk_pct();
  }
  if (proto.has_heat_pct()) {
    cfg.heat_pct = proto.heat_pct();
  }
  if (proto.has_max_units()) {
    cfg.max_units = proto.max_units();
  }
  if (proto.has_stop_atr_mult()) {
    cfg.stop_atr_mult = proto.stop_atr_mult();
  }
  if (proto.has_add_atr_mult()) {
    cfg.add_atr_mult = proto.add_atr_mult();
  }
  if (proto.has_max_weight_pct()) {
    cfg.max_weight_pct = proto.max_weight_pct();
  }
  if (proto.has_min_order_quote()) {
    cfg.min_order_quote = proto.min_order_quote();
  }
  if (const std::string error = sizing::validate(cfg); !error.empty()) {
    std::cerr << "invalid sizing: " << error << '\n';
    exit_config();
  }
  return cfg;
}

void write_sizing_output(const std::vector<Action> &actions,
                         tradingcopilot::v1::RunReport &report) {
  for (const Action &action : actions) {
    auto *out = report.add_sizing();
    out->set_bar_open_ms(action.bar_open_ms);
    out->set_side(std::string(magic_enum::enum_name(action.side)));
    out->set_notional(action.notional);
    out->set_stop_price(action.stop);
    out->set_requested_risk_pct(action.requested_risk_pct);
    out->set_capped_risk_pct(action.capped_risk_pct);
    out->set_rule_id(action.reason);
  }
}

} // namespace dagobert
