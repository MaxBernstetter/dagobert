include(${CMAKE_CURRENT_SOURCE_DIR}/../cmake/Arrow.cmake)

add_library(dagobert_bars_parquet STATIC bars_parquet.cpp)
target_include_directories(
  dagobert_bars_parquet
  PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}
         ${CMAKE_CURRENT_SOURCE_DIR}/../data-models)
target_include_directories(dagobert_bars_parquet SYSTEM PRIVATE
                                           ${DAGOBERT_ARROW_INCLUDE_DIRS})
target_link_libraries(dagobert_bars_parquet PUBLIC dagobert_data_models
                                                  parquet_static arrow_static)
dagobert_strict_warnings(dagobert_bars_parquet)
target_compile_options(dagobert_bars_parquet PRIVATE -Wno-c2y-extensions
                                                    -Wno-deprecated-declarations)
if(DAGOBERT_MODE STREQUAL "debug")
  target_compile_options(
    dagobert_bars_parquet
    PRIVATE -O1 -Wno-\#warnings -Wno-nullability-extension -Wno-gcc-compat)
endif()
