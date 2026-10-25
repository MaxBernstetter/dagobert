add_library(dagobert_indicators STATIC stratforge_smoke.cpp)
target_link_libraries(dagobert_indicators PRIVATE StratForge::stratforge)
dagobert_strict_warnings(dagobert_indicators)
if(DAGOBERT_MODE STREQUAL "debug")
  # Nix clang sets _FORTIFY_SOURCE=2; -O0 in the debug preset triggers a hard error in
  # system headers when StratForge pulls <chrono>.
  target_compile_options(dagobert_indicators PRIVATE -O1)
endif()
