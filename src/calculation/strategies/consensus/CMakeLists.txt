add_library(consensus_strategy STATIC consensus_strategy.cpp)
target_include_directories(
  consensus_strategy
  PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}
         ${CMAKE_CURRENT_SOURCE_DIR}/../../sdk
         ${CMAKE_CURRENT_SOURCE_DIR}/../../sdk/indicators
         ${CMAKE_CURRENT_SOURCE_DIR}/../../sdk/signals
         ${CMAKE_CURRENT_SOURCE_DIR}/../../data-models
         ${CMAKE_CURRENT_SOURCE_DIR}/../../gen/cpp)
target_link_libraries(consensus_strategy PUBLIC dagobert_strategy)
dagobert_strict_warnings(consensus_strategy)
if(DAGOBERT_MODE STREQUAL "debug")
  target_compile_options(
    consensus_strategy
    PRIVATE -O1 -Wno-\#warnings -Wno-nullability-extension -Wno-gcc-compat)
endif()

add_executable(strategy-signal-consensus main.cpp)
target_link_libraries(strategy-signal-consensus PRIVATE consensus_strategy
                                                 dagobert_bars_parquet
                                                 magic_enum::magic_enum)
target_link_options(strategy-signal-consensus PRIVATE -static-libstdc++ -static-libgcc)
dagobert_strict_warnings(strategy-signal-consensus)
if(DAGOBERT_MODE STREQUAL "debug")
  target_compile_options(
    strategy-signal-consensus
    PRIVATE -O1 -Wno-\#warnings -Wno-nullability-extension -Wno-gcc-compat)
endif()

dagobert_register_strategy_run(
  strategy-signal-consensus
  ${CMAKE_CURRENT_SOURCE_DIR}/../../tests/fixtures/consensus_config.json)
