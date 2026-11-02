include(FetchContent)

set(_dagobert_stratforge_sha
    "d25404dcf2c3842ba304ddc60ac19cfec1f8aa37")

FetchContent_Declare(
  stratforge
  GIT_REPOSITORY https://github.com/StratCraftsAI/StratForge.git
  GIT_TAG ${_dagobert_stratforge_sha}
  GIT_SHALLOW FALSE)

set(SF_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(SF_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(SF_BUILD_BENCHMARKS OFF CACHE BOOL "" FORCE)

if(DAGOBERT_MODE STREQUAL "optimized")
  set(SF_ENABLE_SIMD ON CACHE BOOL "" FORCE)
else()
  set(SF_ENABLE_SIMD OFF CACHE BOOL "" FORCE)
endif()

FetchContent_MakeAvailable(stratforge)
