# protobuf + abseil come from the shared prefix (see cmake/DepsKey.cmake).
find_package(Protobuf CONFIG REQUIRED)

set(DAGOBERT_PROTO_GEN_DIR ${CMAKE_CURRENT_SOURCE_DIR}/gen/cpp)
file(
  GLOB_RECURSE
  _dagobert_proto_sources
  CONFIGURE_DEPENDS
  ${DAGOBERT_PROTO_GEN_DIR}/tradingcopilot/v1/*.pb.cc)

add_library(dagobert_proto ${_dagobert_proto_sources})
target_include_directories(dagobert_proto PUBLIC ${DAGOBERT_PROTO_GEN_DIR})
target_link_libraries(dagobert_proto PUBLIC protobuf::libprotobuf)
if(DAGOBERT_MODE STREQUAL "debug")
  target_compile_options(
    dagobert_proto
    PRIVATE -Wno-\#warnings -Wno-nullability-extension -Wno-gcc-compat)
endif()
