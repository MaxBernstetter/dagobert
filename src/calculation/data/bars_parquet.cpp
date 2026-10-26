#include "bars_parquet.hpp"

#include <arrow/array.h>
#include <arrow/io/file.h>
#include <arrow/table.h>
#include <arrow/util/key_value_metadata.h>
#include <parquet/arrow/reader.h>
#include <parquet/metadata.h>

#include <parquet/exception.h>

#include <algorithm>
#include <cctype>
#include <charconv>
#include <chrono>
#include <iterator>
#include <stdexcept>
#include <string>
#include <string_view>
#include <utility>

namespace dagobert {
namespace {

constexpr const char *kKeyIntervalMs = "trading_copilot.interval_ms";

[[nodiscard]] int year_utc(std::int64_t open_time_ms) {
  using namespace std::chrono;
  const auto point = system_clock::time_point{milliseconds{open_time_ms}};
  const year_month_day ymd{floor<days>(point)};
  return static_cast<int>(ymd.year());
}

[[nodiscard]] std::vector<int> years_spanning(std::int64_t from_ms,
                                              std::int64_t to_ms) {
  if (to_ms <= from_ms) {
    return {};
  }
  const int y0 = year_utc(from_ms);
  const int y1 = year_utc(to_ms - 1);
  std::vector<int> years;
  for (int y = y0; y <= y1; ++y) {
    years.push_back(y);
  }
  return years;
}

[[nodiscard]] std::string parquet_path(const std::string &data_dir,
                                       const std::string &provider,
                                       const std::string &base,
                                       const std::string &quote, int year) {
  return data_dir + "/parquet/provider=" + provider + "/base=" + base +
         "/quote=" + quote + "/year=" + std::to_string(year) +
         "/candles.parquet";
}

[[nodiscard]] std::int64_t footer_interval_ms(
    const std::shared_ptr<const parquet::FileMetaData> &metadata) {
  if (metadata == nullptr || metadata->key_value_metadata() == nullptr) {
    return 0;
  }
  const std::shared_ptr<const arrow::KeyValueMetadata> &kvm =
      metadata->key_value_metadata();
  const int idx = kvm->FindKey(kKeyIntervalMs);
  if (idx < 0) {
    return 0;
  }
  try {
    return std::stoll(kvm->value(idx));
  } catch (...) {
    return 0;
  }
}

// Same contract as std::stod: leading whitespace and '+' are skipped, trailing
// characters are ignored, no digits throws invalid_argument, overflow throws
// out_of_range. std::from_chars avoids the locale and strtod cost.
[[nodiscard]] double parse_decimal_string(std::string_view value) {
  const char *first = value.data();
  const char *const last = first + value.size();
  while (first != last && std::isspace(static_cast<unsigned char>(*first))) {
    ++first;
  }
  if (first != last && *first == '+') {
    ++first;
    if (first != last && (*first == '-' || *first == '+')) {
      throw std::invalid_argument("stod");
    }
  }
  double out = 0.0;
  const auto res = std::from_chars(first, last, out);
  if (res.ec == std::errc::invalid_argument) {
    throw std::invalid_argument("stod");
  }
  if (res.ec == std::errc::result_out_of_range) {
    throw std::out_of_range("stod");
  }
  return out;
}

[[nodiscard]] const arrow::StringArray *
string_column(const std::shared_ptr<arrow::Table> &table,
              const std::string &name) {
  const std::shared_ptr<arrow::ChunkedArray> chunked =
      table->GetColumnByName(name);
  if (chunked == nullptr || chunked->num_chunks() != 1) {
    throw std::runtime_error("missing or chunked column: " + name);
  }
  return dynamic_cast<const arrow::StringArray *>(chunked->chunk(0).get());
}

[[nodiscard]] const arrow::Int64Array *
int64_column(const std::shared_ptr<arrow::Table> &table,
             const std::string &name) {
  const std::shared_ptr<arrow::ChunkedArray> chunked =
      table->GetColumnByName(name);
  if (chunked == nullptr || chunked->num_chunks() != 1) {
    throw std::runtime_error("missing or chunked column: " + name);
  }
  return dynamic_cast<const arrow::Int64Array *>(chunked->chunk(0).get());
}

[[nodiscard]] std::vector<Bar> read_parquet_file(const std::string &path,
                                                 std::int64_t from_ms,
                                                 std::int64_t to_ms) {
  arrow::MemoryPool *pool = arrow::default_memory_pool();
  std::shared_ptr<arrow::io::ReadableFile> input;
  PARQUET_ASSIGN_OR_THROW(input, arrow::io::ReadableFile::Open(path));

  std::unique_ptr<parquet::arrow::FileReader> reader;
  PARQUET_THROW_NOT_OK(parquet::arrow::OpenFile(input, pool, &reader));

  std::shared_ptr<arrow::Table> table;
  // One small file: a thread pool costs more than it saves.
  reader->set_use_threads(false);
  PARQUET_THROW_NOT_OK(reader->ReadTable(&table));

  const std::int64_t footer_interval =
      footer_interval_ms(reader->parquet_reader()->metadata());

  const auto *open_time = int64_column(table, "open_time");
  const auto *open = string_column(table, "open");
  const auto *high = string_column(table, "high");
  const auto *low = string_column(table, "low");
  const auto *close = string_column(table, "close");
  const auto *volume = string_column(table, "volume");
  const auto *close_time = int64_column(table, "close_time");

  const int64_t row_count = table->num_rows();
  std::vector<Bar> bars;
  bars.reserve(static_cast<std::size_t>(row_count));

  for (int64_t i = 0; i < row_count; ++i) {
    if (open_time->IsNull(i)) {
      continue;
    }
    const std::int64_t open_ms = open_time->Value(i);
    if (open_ms < from_ms || open_ms >= to_ms) {
      continue;
    }
    Bar bar;
    bar.open_ms = open_ms;
    bar.open = parse_decimal_string(open->GetView(i));
    bar.high = parse_decimal_string(high->GetView(i));
    bar.low = parse_decimal_string(low->GetView(i));
    bar.close = parse_decimal_string(close->GetView(i));
    bar.volume = parse_decimal_string(volume->GetView(i));
    if (!close_time->IsNull(i)) {
      const std::int64_t close_ms = close_time->Value(i);
      // Binance close_time is inclusive: open + interval - 1 ms.
      bar.duration_ms = close_ms - open_ms + 1;
      if (bar.duration_ms <= 0 && footer_interval > 0) {
        bar.duration_ms = footer_interval;
      }
    } else if (footer_interval > 0) {
      bar.duration_ms = footer_interval;
    }
    bars.push_back(std::move(bar));
  }

  std::sort(bars.begin(), bars.end(),
            [](const Bar &a, const Bar &b) { return a.open_ms < b.open_ms; });

  for (std::size_t i = 0; i < bars.size(); ++i) {
    if (i + 1U < bars.size()) {
      const std::int64_t spacing = bars[i + 1].open_ms - bars[i].open_ms;
      if (spacing > 0) {
        bars[i].duration_ms = spacing;
      }
    } else if (bars[i].duration_ms <= 0 && footer_interval > 0) {
      bars[i].duration_ms = footer_interval;
    }
  }

  return bars;
}

} // namespace

std::vector<Bar> load_bars_from_lake(const std::string &data_dir,
                                     const std::string &provider,
                                     const std::string &base,
                                     const std::string &quote,
                                     std::int64_t from_ms, std::int64_t to_ms) {
  std::vector<Bar> all;
  for (const int year : years_spanning(from_ms, to_ms)) {
    const std::string path =
        parquet_path(data_dir, provider, base, quote, year);
    std::vector<Bar> part = read_parquet_file(path, from_ms, to_ms);
    if (all.empty()) {
      all = std::move(part);
    } else {
      all.insert(all.end(), std::make_move_iterator(part.begin()),
                 std::make_move_iterator(part.end()));
    }
  }
  std::sort(all.begin(), all.end(),
            [](const Bar &a, const Bar &b) { return a.open_ms < b.open_ms; });
  return all;
}

} // namespace dagobert
