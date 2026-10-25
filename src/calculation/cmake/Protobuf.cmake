include(FetchContent)

set(_dagobert_protobuf_tag "v36.2")

set(protobuf_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(protobuf_BUILD_PROTOC_BINARIES OFF CACHE BOOL "" FORCE)
set(protobuf_INSTALL OFF CACHE BOOL "" FORCE)
set(protobuf_ABSL_PROVIDER "module" CACHE STRING "" FORCE)
set(ABSL_PROPAGATE_CXX_STD ON CACHE BOOL "" FORCE)
set(ABSL_ENABLE_INSTALL OFF CACHE BOOL "" FORCE)

FetchContent_Declare(
  protobuf
  GIT_REPOSITORY https://github.com/protocolbuffers/protobuf.git
  GIT_TAG ${_dagobert_protobuf_tag}
  GIT_SHALLOW TRUE)

FetchContent_MakeAvailable(protobuf)

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
