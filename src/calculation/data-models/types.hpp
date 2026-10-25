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
};

struct Fill {
  std::int64_t ts_ms{};
  std::string side;
  double qty{};
  double price{};
  std::string reason;
};

} // namespace dagobert
