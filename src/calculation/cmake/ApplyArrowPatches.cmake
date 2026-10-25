cmake_minimum_required(VERSION 3.25)
if(NOT DEFINED ARROW_TOOLCHAIN)
  message(FATAL_ERROR "ARROW_TOOLCHAIN not set")
endif()
if(NOT EXISTS "${ARROW_TOOLCHAIN}")
  message(FATAL_ERROR "Arrow toolchain file not found: ${ARROW_TOOLCHAIN}")
endif()
file(READ "${ARROW_TOOLCHAIN}" _dagobert_arrow_toolchain)
set(_dagobert_arrow_patched "${_dagobert_arrow_toolchain}")
string(REPLACE
       "https://dlcdn.apache.org/thrift/"
       "https://archive.apache.org/dist/thrift/"
       _dagobert_arrow_patched "${_dagobert_arrow_patched}")
if(NOT _dagobert_arrow_patched MATCHES "CMAKE_POLICY_VERSION_MINIMUM=3.5")
  string(REPLACE
         [==[-DCMAKE_VERBOSE_MAKEFILE=${CMAKE_VERBOSE_MAKEFILE})]==]
         [==[-DCMAKE_VERBOSE_MAKEFILE=${CMAKE_VERBOSE_MAKEFILE} -DCMAKE_POLICY_VERSION_MINIMUM=3.5)]==]
         _dagobert_arrow_patched "${_dagobert_arrow_patched}")
endif()
if(NOT _dagobert_arrow_patched STREQUAL _dagobert_arrow_toolchain)
  file(WRITE "${ARROW_TOOLCHAIN}" "${_dagobert_arrow_patched}")
endif()
