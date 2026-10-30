add_executable(dagobert-tests magic_enum_test.cpp quote_test.cpp position_test.cpp
                                   sma_test.cpp ema_rsi_test.cpp atr_test.cpp
                                   macd_test.cpp close_distance_test.cpp
                                   sma_cross_signal_test.cpp consensus_gates_test.cpp consensus_signal_test.cpp
                                   sma_cross_strategy_test.cpp
                                   bars_parquet_test.cpp sizing_test.cpp
                                   sma_cross_exe_test.cpp sma_cross_sizing_test.cpp
                                   consensus_strategy_test.cpp
                                   consensus_exe_test.cpp consensus_equivalence_test.cpp sweep_exe_test.cpp)
target_include_directories(
  dagobert-tests
  PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/../data-models
          ${CMAKE_CURRENT_SOURCE_DIR}/../sdk/indicators
          ${CMAKE_CURRENT_SOURCE_DIR}/../sdk/signals
          ${CMAKE_CURRENT_SOURCE_DIR}/../sdk
          ${CMAKE_CURRENT_SOURCE_DIR}/../strategies/sma-cross
          ${CMAKE_CURRENT_SOURCE_DIR}/../strategies/consensus
          ${CMAKE_CURRENT_SOURCE_DIR}/../gen/cpp)
target_link_libraries(
  dagobert-tests
  PRIVATE GTest::gtest_main magic_enum::magic_enum dagobert_data_models
          dagobert_indicators dagobert_signals dagobert_strategy
          sma_cross_strategy consensus_strategy dagobert_proto dagobert_bars_parquet)
add_dependencies(dagobert-tests strategy-sma-cross strategy-signal-consensus)
target_compile_definitions(dagobert-tests
                           PRIVATE SMA_CROSS_BIN="$<TARGET_FILE:strategy-sma-cross>"
                           CONSENSUS_BIN="$<TARGET_FILE:strategy-signal-consensus>")
dagobert_strict_warnings(dagobert-tests)
# Precompiled header: gtest and the standard headers shared by the test TUs
# are parsed once. Keep it to headers every test already includes.
target_precompile_headers(
  dagobert-tests
  PRIVATE <gtest/gtest.h> <cstdint> <functional> <memory> <optional>
          <ostream> <span> <string> <vector>)
# Clang stamps the PCH with its build time, so the file differs on every build
# and ccache misses every TU that includes it. This flag drops the stamp.
target_compile_options(dagobert-tests PRIVATE -Xclang -fno-pch-timestamp)
target_link_options(dagobert-tests PRIVATE -static-libstdc++ -static-libgcc)
if(DAGOBERT_MODE STREQUAL "debug")
  target_compile_options(
    dagobert-tests
    PRIVATE -O1 -Wno-\#warnings -Wno-nullability-extension -Wno-gcc-compat)
endif()

include(GoogleTest)
gtest_discover_tests(dagobert-tests DISCOVERY_MODE PRE_TEST)
target_compile_definitions(
  dagobert-tests
  PRIVATE DAGOBERT_TEST_FIXTURES_DIR="${CMAKE_CURRENT_SOURCE_DIR}/fixtures")
target_compile_definitions(
  dagobert-tests
  PRIVATE SIZING_VECTORS_PATH="${CMAKE_CURRENT_SOURCE_DIR}/../../../src/calculation-signal/playbooks/position-sizing/position-sizing-vectors.json")
target_compile_definitions(
  dagobert-tests
  PRIVATE SMA_CROSS_SIZING_FIXTURE_PATH="${CMAKE_CURRENT_SOURCE_DIR}/fixtures/sma_cross_sizing.json")
target_compile_definitions(
  dagobert-tests
  PRIVATE CONSENSUS_FIXTURE_PATH="${CMAKE_CURRENT_SOURCE_DIR}/fixtures/consensus_sizing.json"
          CONSENSUS_PARAMS_FIXTURE_PATH="${CMAKE_CURRENT_SOURCE_DIR}/fixtures/consensus_params.json"
          CONSENSUS_EQUIVALENCE_FIXTURE_PATH="${CMAKE_CURRENT_SOURCE_DIR}/fixtures/consensus_equivalence.json")
