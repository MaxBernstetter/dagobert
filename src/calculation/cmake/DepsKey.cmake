# Locates the shared, pre-built dependency prefix (Arrow, Parquet, protobuf,
# abseil, thrift, snappy, zlib). Included by the main build and by deps/.
# The key covers everything that changes the binaries: the deps recipe
# (deps/CMakeLists.txt, patches, this file), compiler id+version and the
# build type.
if(NOT DEFINED DAGOBERT_DEPS_ROOT)
  if(DEFINED ENV{DAGOBERT_DEPS_ROOT})
    set(DAGOBERT_DEPS_ROOT "$ENV{DAGOBERT_DEPS_ROOT}")
  elseif(DEFINED ENV{XDG_CACHE_HOME})
    set(DAGOBERT_DEPS_ROOT "$ENV{XDG_CACHE_HOME}/dagobert/deps")
  else()
    set(DAGOBERT_DEPS_ROOT "$ENV{HOME}/.cache/dagobert/deps")
  endif()
endif()

set(_dagobert_key_material
    "${CMAKE_CXX_COMPILER_ID}-${CMAKE_CXX_COMPILER_VERSION}-${CMAKE_BUILD_TYPE}")
if(DAGOBERT_PORTABLE)
  # The portable prefix is built with gcc and linked by clang (container
  # image), so the compiler must not be part of its key.
  set(_dagobert_key_material "portable-${CMAKE_BUILD_TYPE}")
endif()
foreach(_f deps/CMakeLists.txt cmake/ApplyArrowPatches.cmake cmake/DepsKey.cmake)
  file(SHA256 "${DAGOBERT_CALC_ROOT}/${_f}" _h)
  string(APPEND _dagobert_key_material "-${_h}")
endforeach()
string(SHA256 _dagobert_key "${_dagobert_key_material}")
string(SUBSTRING "${_dagobert_key}" 0 12 _dagobert_key)
string(TOLOWER "${CMAKE_BUILD_TYPE}" _dagobert_type)
set(DAGOBERT_DEPS_KEY "${_dagobert_type}-${_dagobert_key}")
if(DAGOBERT_PORTABLE)
  set(DAGOBERT_DEPS_KEY "${_dagobert_type}-portable-${_dagobert_key}")
endif()
set(DAGOBERT_DEPS_PREFIX "${DAGOBERT_DEPS_ROOT}/${DAGOBERT_DEPS_KEY}")
