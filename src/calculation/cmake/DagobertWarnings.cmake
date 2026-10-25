function(dagobert_strict_warnings target)
  if(DAGOBERT_MODE STREQUAL "debug")
    # Nix clang sets _FORTIFY_SOURCE=2; -O0 turns glibc #warnings into errors.
    target_compile_options(${target} PRIVATE -Wno-\#warnings)
  endif()
  target_compile_options(
    ${target}
    PRIVATE -Wall -Wextra -Wpedantic -Werror -Wno-nullability-extension)
endfunction()
