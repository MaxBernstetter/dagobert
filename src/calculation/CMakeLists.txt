cmake_minimum_required(VERSION 3.25)

if(NOT DAGOBERT_MODE STREQUAL "debug" AND NOT DAGOBERT_MODE STREQUAL "optimized")
  message(
    FATAL_ERROR
    "DAGOBERT_MODE must be 'debug' or 'optimized' (got '${DAGOBERT_MODE}')")
endif()

set(CMAKE_CXX_STANDARD 23)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

include(${CMAKE_CURRENT_SOURCE_DIR}/cmake/Ccache.cmake)

project(dagobert_calculation LANGUAGES C CXX)

# Arrow/Parquet/protobuf come from the shared prefix built by `task cpp:deps`.
set(DAGOBERT_CALC_ROOT ${CMAKE_CURRENT_SOURCE_DIR})
include(${CMAKE_CURRENT_SOURCE_DIR}/cmake/DepsKey.cmake)
if(NOT EXISTS "${DAGOBERT_DEPS_PREFIX}/.complete")
  message(
    FATAL_ERROR
    "Dependencies are not built: missing ${DAGOBERT_DEPS_PREFIX}\n"
    "Builds never compile dependencies. Run once:  task cpp:deps PRESET=${DAGOBERT_MODE}")
endif()
list(PREPEND CMAKE_PREFIX_PATH "${DAGOBERT_DEPS_PREFIX}")

if(DAGOBERT_MODE STREQUAL "debug")
  set(CMAKE_EXPORT_COMPILE_COMMANDS ON)
endif()

set(CMAKE_POSITION_INDEPENDENT_CODE ON)
set(CMAKE_BUILD_RPATH "$ORIGIN")

include(${CMAKE_CURRENT_SOURCE_DIR}/cmake/DagobertWarnings.cmake)

set(CMAKE_EXE_LINKER_FLAGS "${CMAKE_EXE_LINKER_FLAGS} -fuse-ld=lld")
set(CMAKE_SHARED_LINKER_FLAGS "${CMAKE_SHARED_LINKER_FLAGS} -fuse-ld=lld")
set(CMAKE_MODULE_LINKER_FLAGS "${CMAKE_MODULE_LINKER_FLAGS} -fuse-ld=lld")

if(DAGOBERT_MODE STREQUAL "debug")
  set(_dagobert_debug_flags
      "-O0 -g3 -fno-omit-frame-pointer -fno-inline -fno-optimize-sibling-calls -fstandalone-debug -U_LIBCPP_HARDENING_MODE -D_LIBCPP_HARDENING_MODE=_LIBCPP_HARDENING_MODE_DEBUG"
  )
  set(CMAKE_C_FLAGS_DEBUG "${_dagobert_debug_flags}")
  set(CMAKE_CXX_FLAGS_DEBUG "${_dagobert_debug_flags}")
  set(CMAKE_EXE_LINKER_FLAGS_DEBUG "-fuse-ld=lld")
  set(CMAKE_SHARED_LINKER_FLAGS_DEBUG "-fuse-ld=lld")
elseif(DAGOBERT_MODE STREQUAL "optimized")
  # DAGOBERT_PORTABLE=ON (container images): baseline x86-64/arm64 code, runs on
  # any CPU of the architecture. Default: tuned for the build host.
  if(DAGOBERT_PORTABLE)
    set(_dagobert_cpu "-mtune=generic")
  else()
    set(_dagobert_cpu "-march=native -mtune=native")
  endif()
  set(CMAKE_C_FLAGS_RELEASE
      "-O3 -DNDEBUG ${_dagobert_cpu} -flto=thin -ffunction-sections -fdata-sections -g -fno-omit-frame-pointer"
  )
  set(CMAKE_CXX_FLAGS_RELEASE
      "-O3 -DNDEBUG ${_dagobert_cpu} -flto=thin -ffunction-sections -fdata-sections -g -fno-omit-frame-pointer"
  )
  set(CMAKE_EXE_LINKER_FLAGS_RELEASE
      "-flto=thin -fuse-ld=lld -Wl,--gc-sections -Wl,--as-needed -Wl,-O2")
  set(CMAKE_SHARED_LINKER_FLAGS_RELEASE
      "-flto=thin -fuse-ld=lld -Wl,--gc-sections -Wl,--as-needed -Wl,-O2")
endif()

include(${CMAKE_CURRENT_SOURCE_DIR}/cmake/Valgrind.cmake)
include(${CMAKE_CURRENT_SOURCE_DIR}/cmake/StratForge.cmake)
include(${CMAKE_CURRENT_SOURCE_DIR}/cmake/MagicEnum.cmake)

add_library(dagobert_sdk OBJECT sdk/dagobert.cpp)
target_include_directories(dagobert_sdk PUBLIC sdk)
dagobert_strict_warnings(dagobert_sdk)

add_subdirectory(sdk/indicators)
add_subdirectory(sdk/signals)
add_subdirectory(data-models)

include(${CMAKE_CURRENT_SOURCE_DIR}/cmake/Protobuf.cmake)

add_subdirectory(sdk)
add_subdirectory(strategies)
add_subdirectory(data)

add_library(dagobert INTERFACE)
target_compile_features(dagobert INTERFACE cxx_std_23)
target_link_libraries(dagobert INTERFACE dagobert_sdk)

include(${CMAKE_CURRENT_SOURCE_DIR}/cmake/GoogleTest.cmake)

enable_testing()
add_subdirectory(tests)

dagobert_define_valgrind_targets()
