include(FetchContent)

set(_dagobert_magic_enum_sha
    "5de969c9005938a27a34d3af3ea5923d18679ad8")

FetchContent_Declare(
  magic_enum
  GIT_REPOSITORY https://github.com/Neargye/magic_enum.git
  GIT_TAG ${_dagobert_magic_enum_sha}
  GIT_SHALLOW FALSE)

FetchContent_MakeAvailable(magic_enum)
