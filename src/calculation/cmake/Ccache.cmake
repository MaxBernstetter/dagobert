# Use ccache (through cmake/ccache-launcher.sh) when installed. Include
# before project(). Set CCACHE_DISABLE=1 in the environment to bypass it.
find_program(DAGOBERT_CCACHE ccache)
if(DAGOBERT_CCACHE)
  set(CMAKE_C_COMPILER_LAUNCHER "${CMAKE_CURRENT_LIST_DIR}/ccache-launcher.sh")
  set(CMAKE_CXX_COMPILER_LAUNCHER "${CMAKE_C_COMPILER_LAUNCHER}")
endif()
