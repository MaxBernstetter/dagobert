add_library(sma_cross_strategy STATIC sma_cross_strategy.cpp)
target_include_directories(
  sma_cross_strategy
  PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}
         ${CMAKE_CURRENT_SOURCE_DIR}/../../sdk
         ${CMAKE_CURRENT_SOURCE_DIR}/../../sdk/indicators
         ${CMAKE_CURRENT_SOURCE_DIR}/../../sdk/signals
         ${CMAKE_CURRENT_SOURCE_DIR}/../../data-models
         ${CMAKE_CURRENT_SOURCE_DIR}/../../gen/cpp)
target_link_libraries(sma_cross_strategy PUBLIC dagobert_strategy)
dagobert_strict_warnings(sma_cross_strategy)
if(DAGOBERT_MODE STREQUAL "debug")
  target_compile_options(
    sma_cross_strategy
    PRIVATE -O1 -Wno-\#warnings -Wno-nullability-extension -Wno-gcc-compat)
endif()
