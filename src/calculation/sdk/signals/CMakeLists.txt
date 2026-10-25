add_library(dagobert_signals STATIC sma_cross_signal.cpp consensus_gates.cpp
                                 consensus_signal.cpp)
target_include_directories(
  dagobert_signals
  PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}
         ${CMAKE_CURRENT_SOURCE_DIR}/../../data-models)
target_link_libraries(dagobert_signals PUBLIC dagobert_data_models)
dagobert_strict_warnings(dagobert_signals)
if(DAGOBERT_MODE STREQUAL "debug")
  target_compile_options(dagobert_signals PRIVATE -O1)
endif()
