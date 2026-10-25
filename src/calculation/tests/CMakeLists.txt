add_executable(dagobert-tests magic_enum_test.cpp quote_test.cpp position_test.cpp
                                   sma_test.cpp ema_rsi_test.cpp atr_test.cpp)
target_include_directories(
  dagobert-tests
  PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/../data-models
          ${CMAKE_CURRENT_SOURCE_DIR}/../sdk/indicators)
target_link_libraries(
  dagobert-tests
  PRIVATE GTest::gtest_main magic_enum::magic_enum dagobert_data_models
          dagobert_indicators)
dagobert_strict_warnings(dagobert-tests)
target_link_options(dagobert-tests PRIVATE -static-libstdc++ -static-libgcc)
if(DAGOBERT_MODE STREQUAL "debug")
  target_compile_options(dagobert-tests PRIVATE -O1)
endif()

include(GoogleTest)
gtest_discover_tests(dagobert-tests DISCOVERY_MODE PRE_TEST)
