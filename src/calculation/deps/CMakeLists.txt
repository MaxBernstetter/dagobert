# Builds the external dependencies once into a shared install prefix
# (DAGOBERT_DEPS_ROOT/<key>). Driven by deps/build.sh via `task cpp:deps`;
# do not run it from the main build. Editing this file changes the key, so
# all prefixes are rebuilt on the next `task cpp:deps`.
cmake_minimum_required(VERSION 3.25)

if(NOT DAGOBERT_MODE STREQUAL "debug" AND NOT DAGOBERT_MODE STREQUAL "optimized")
  message(FATAL_ERROR "DAGOBERT_MODE must be 'debug' or 'optimized'")
endif()

project(dagobert_deps LANGUAGES C CXX)

get_filename_component(DAGOBERT_CALC_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/.." ABSOLUTE)
include(${DAGOBERT_CALC_ROOT}/cmake/Ccache.cmake)
include(${DAGOBERT_CALC_ROOT}/cmake/DepsKey.cmake)
include(ExternalProject)

file(WRITE "${CMAKE_BINARY_DIR}/key.txt" "${DAGOBERT_DEPS_KEY}")

set(_install "${DAGOBERT_DEPS_PREFIX}")
if(DAGOBERT_DEPS_STAGING)
  set(_install "${DAGOBERT_DEPS_PREFIX}.partial")
endif()

set(_arrow_tag "apache-arrow-18.1.0")
set(_protobuf_tag "v36.2")

# Same flags as the main build (CMakeLists.txt) so objects are compatible.
if(DAGOBERT_MODE STREQUAL "debug")
  set(_main_flags
      "-O0 -g3 -fno-omit-frame-pointer -fno-inline -fno-optimize-sibling-calls -fstandalone-debug -U_LIBCPP_HARDENING_MODE -D_LIBCPP_HARDENING_MODE=_LIBCPP_HARDENING_MODE_DEBUG")
  # Arrow debug builds use -Werror (CHECKIN); Nix _FORTIFY_SOURCE needs -O.
  set(_arrow_flags "-O1 -g3 -Wno-\\#warnings")
else()
  if(DAGOBERT_PORTABLE)
    # Container image: baseline CPU, no LTO (gcc-built objects must link into
    # the clang-built executables, and LTO bitcode is not interchangeable).
    set(_main_flags
        "-O3 -DNDEBUG -mtune=generic -ffunction-sections -fdata-sections -fno-omit-frame-pointer")
  else()
    set(_main_flags
        "-O3 -DNDEBUG -march=native -mtune=native -flto=thin -ffunction-sections -fdata-sections -g -fno-omit-frame-pointer")
  endif()
  set(_arrow_flags "${_main_flags}")
endif()

set(_launcher "${CMAKE_CXX_COMPILER_LAUNCHER}")

set(_common
    -DCMAKE_BUILD_TYPE=${CMAKE_BUILD_TYPE}
    -DCMAKE_C_COMPILER=${CMAKE_C_COMPILER}
    -DCMAKE_CXX_COMPILER=${CMAKE_CXX_COMPILER}
    -DCMAKE_C_COMPILER_LAUNCHER=${_launcher}
    -DCMAKE_CXX_COMPILER_LAUNCHER=${_launcher}
    -DCMAKE_CXX_STANDARD=23
    -DCMAKE_POSITION_INDEPENDENT_CODE=ON
    -DCMAKE_INSTALL_PREFIX=${_install}
    -DCMAKE_INSTALL_LIBDIR=lib
    -DCMAKE_EXE_LINKER_FLAGS=-fuse-ld=lld
    -DCMAKE_SHARED_LINKER_FLAGS=-fuse-ld=lld
    -DCMAKE_POLICY_VERSION_MINIMUM=3.5)

string(TOUPPER "${CMAKE_BUILD_TYPE}" _bt)

ExternalProject_Add(
  protobuf
  GIT_REPOSITORY https://github.com/protocolbuffers/protobuf.git
  GIT_TAG ${_protobuf_tag}
  GIT_SHALLOW TRUE
  LIST_SEPARATOR "|"
  CMAKE_ARGS
    ${_common}
    -DCMAKE_C_FLAGS_${_bt}=${_main_flags}
    -DCMAKE_CXX_FLAGS_${_bt}=${_main_flags}
    -Dprotobuf_BUILD_TESTS=OFF
    -Dprotobuf_BUILD_PROTOC_BINARIES=OFF
    -Dprotobuf_INSTALL=ON
    -Dprotobuf_ABSL_PROVIDER=module
    -DABSL_PROPAGATE_CXX_STD=ON
    -DABSL_ENABLE_INSTALL=ON)

ExternalProject_Add(
  arrow
  GIT_REPOSITORY https://github.com/apache/arrow.git
  GIT_TAG ${_arrow_tag}
  GIT_SHALLOW TRUE
  PATCH_COMMAND
    ${CMAKE_COMMAND}
    -DARROW_TOOLCHAIN=cpp/cmake_modules/ThirdpartyToolchain.cmake
    -P ${DAGOBERT_CALC_ROOT}/cmake/ApplyArrowPatches.cmake
  SOURCE_SUBDIR cpp
  LIST_SEPARATOR "|"
  CMAKE_ARGS
    ${_common}
    -DCMAKE_C_FLAGS_${_bt}=${_arrow_flags}
    -DCMAKE_CXX_FLAGS_${_bt}=${_arrow_flags}
    -DBUILD_WARNING_LEVEL=PRODUCTION
    -DARROW_BUILD_SHARED=OFF
    -DARROW_BUILD_STATIC=ON
    -DARROW_PARQUET=ON
    -DARROW_BUILD_TESTS=OFF
    -DARROW_BUILD_EXAMPLES=OFF
    -DARROW_COMPUTE=OFF
    -DARROW_DATASET=OFF
    -DARROW_FILESYSTEM=OFF
    -DARROW_JSON=OFF
    -DARROW_CSV=OFF
    -DARROW_IPC=ON
    -DARROW_GANDIVA=OFF
    -DARROW_ACERO=OFF
    -DARROW_S3=OFF
    -DARROW_HDFS=OFF
    -DARROW_WITH_THRIFT=ON
    -DARROW_SIMD_LEVEL=NONE
    -DARROW_RUNTIME_SIMD_LEVEL=NONE
    -DARROW_WITH_SNAPPY=ON
    -DARROW_WITH_ZLIB=ON
    -DARROW_WITH_BROTLI=OFF
    -DARROW_WITH_LZ4=OFF
    -DARROW_WITH_ZSTD=OFF
    -DARROW_DEPENDENCY_SOURCE=AUTO)
