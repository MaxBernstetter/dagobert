include(FetchContent)

set(_dagobert_gtest_sha
    "cd8e73f067129f8489e35c7b51c7a68552b289d2")

set(BUILD_GMOCK OFF CACHE BOOL "" FORCE)
set(INSTALL_GTEST OFF CACHE BOOL "" FORCE)
set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)

FetchContent_Declare(
  googletest
  GIT_REPOSITORY https://github.com/google/googletest.git
  GIT_TAG ${_dagobert_gtest_sha}
  GIT_SHALLOW FALSE)

FetchContent_MakeAvailable(googletest)

if(DAGOBERT_MODE STREQUAL "debug")
  foreach(_dagobert_gtest_target gtest gtest_main)
    if(TARGET ${_dagobert_gtest_target})
      target_compile_options(${_dagobert_gtest_target} PRIVATE -O1)
    endif()
  endforeach()
endif()
