add_library(
  dagobert_strategy STATIC strategy.cpp series.cpp sizing_proto.cpp)
target_include_directories(
  dagobert_strategy
  PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}
         ${CMAKE_CURRENT_SOURCE_DIR}/../data-models
         ${CMAKE_CURRENT_SOURCE_DIR}/../gen/cpp)
target_link_libraries(
  dagobert_strategy
  PUBLIC dagobert_data_models dagobert_indicators dagobert_signals
         dagobert_proto)
dagobert_strict_warnings(dagobert_strategy)
if(DAGOBERT_MODE STREQUAL "debug")
  target_compile_options(
    dagobert_strategy
    PRIVATE -O1 -Wno-\#warnings -Wno-nullability-extension -Wno-gcc-compat)
endif()
