# Arrow/Parquet are pre-built in the shared prefix (see cmake/DepsKey.cmake).
# The package config provides the arrow_static and parquet_static targets.
find_package(Arrow CONFIG REQUIRED)
find_package(Parquet CONFIG REQUIRED)

get_target_property(DAGOBERT_ARROW_INCLUDE_DIRS arrow_static
                    INTERFACE_INCLUDE_DIRECTORIES)
