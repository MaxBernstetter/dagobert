# Opt-in profiling targets, not part of ALL.
#   valgrind  memcheck, debug preset only
#   callgrind callgrind + callgrind_annotate, optimized preset only
# Strategy executables call dagobert_register_strategy_run(); the top-level
# CMakeLists calls dagobert_define_valgrind_targets() once, after all of them.
# Outputs land in the build directory. Invoking the target of the other preset
# fails with a message.

find_program(DAGOBERT_VALGRIND valgrind)
find_program(DAGOBERT_CALLGRIND_ANNOTATE callgrind_annotate)

set(DAGOBERT_FIXTURE_LAKE
    ${CMAKE_CURRENT_LIST_DIR}/../tests/fixtures/lake_roundtrip)

# dagobert_register_strategy_run(<exe-target> <config.json>)
function(dagobert_register_strategy_run exe config)
  set_property(GLOBAL APPEND PROPERTY DAGOBERT_STRATEGY_RUNS "${exe}|${config}")
endfunction()

function(dagobert_define_valgrind_targets)
  if(DAGOBERT_MODE STREQUAL "debug")
    set(wrong callgrind)
    set(need optimized)
  else()
    set(wrong valgrind)
    set(need debug)
  endif()
  add_custom_target(
    ${wrong}
    COMMAND ${CMAKE_COMMAND} -E echo
            "${wrong} needs the ${need} preset (this build is ${DAGOBERT_MODE})"
    COMMAND ${CMAKE_COMMAND} -E false)

  if(NOT DAGOBERT_VALGRIND)
    message(STATUS "valgrind not found: valgrind/callgrind targets disabled")
    return()
  endif()

  get_property(runs GLOBAL PROPERTY DAGOBERT_STRATEGY_RUNS)
  set(vg_flags --leak-check=full --show-leak-kinds=definite,indirect
               --errors-for-leak-kinds=definite,indirect --track-origins=yes
               --error-exitcode=1)
  set(commands)
  set(deps)
  if(DAGOBERT_MODE STREQUAL "debug")
    list(APPEND commands
         COMMAND ${DAGOBERT_VALGRIND} --tool=memcheck ${vg_flags}
                 $<TARGET_FILE:dagobert-tests> --gtest_filter=-SmaCrossExe*:*System*)
    list(APPEND deps dagobert-tests)
    foreach(run IN LISTS runs)
      string(REPLACE "|" ";" parts "${run}")
      list(GET parts 0 exe)
      list(GET parts 1 config)
      list(APPEND commands
           COMMAND ${DAGOBERT_VALGRIND} --tool=memcheck ${vg_flags}
                   $<TARGET_FILE:${exe}> --config ${config} --data-dir
                   ${DAGOBERT_FIXTURE_LAKE} > /dev/null)
      list(APPEND deps ${exe})
    endforeach()
    add_custom_target(
      valgrind ${commands}
      DEPENDS ${deps}
      WORKING_DIRECTORY ${CMAKE_BINARY_DIR}
      COMMENT "memcheck: dagobert-tests and strategies")
  else()
    foreach(run IN LISTS runs)
      string(REPLACE "|" ";" parts "${run}")
      list(GET parts 0 exe)
      list(GET parts 1 config)
      set(out ${CMAKE_BINARY_DIR}/callgrind.${exe}.out)
      list(APPEND commands
           COMMAND ${DAGOBERT_VALGRIND} --tool=callgrind --dump-instr=yes
                   --collect-jumps=yes --callgrind-out-file=${out}
                   $<TARGET_FILE:${exe}> --config ${config} --data-dir
                   ${DAGOBERT_FIXTURE_LAKE} > /dev/null
           COMMAND ${DAGOBERT_CALLGRIND_ANNOTATE} --auto=yes ${out} > ${out}.txt)
      list(APPEND deps ${exe})
    endforeach()
    add_custom_target(
      callgrind ${commands}
      DEPENDS ${deps}
      WORKING_DIRECTORY ${CMAKE_BINARY_DIR}
      COMMENT "callgrind: strategies (annotation in callgrind.<exe>.out.txt)")
  endif()
endfunction()
