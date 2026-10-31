cmake_minimum_required(VERSION 3.20)
project(easyDVD_ffms2 LANGUAGES C CXX)
if(NOT MSVC OR NOT CMAKE_SIZEOF_VOID_P EQUAL 4)
  message(FATAL_ERROR "The HCEnc support plugin requires the 32-bit MSVC ABI.")
endif()
set(CMAKE_CXX_STANDARD 11)
file(GLOB CORE_SOURCES "${FFMS_SOURCE}/src/core/*.cpp")
add_library(ffms2 SHARED ${CORE_SOURCES}
  "${FFMS_SOURCE}/src/avisynth/avisynth.cpp" "${FFMS_SOURCE}/src/avisynth/avssources.cpp")
target_include_directories(ffms2 PRIVATE "${FFMS_SOURCE}/include"
  "${AVS_SOURCE}/avs_core/include" "${FFMPEG_PREFIX}/include")
target_compile_definitions(ffms2 PRIVATE FFMS_EXPORTS __STDC_CONSTANT_MACROS
  _FILE_OFFSET_BITS=64 _CRT_SECURE_NO_WARNINGS)
target_compile_options(ffms2 PRIVATE /EHsc)
target_link_libraries(ffms2 PRIVATE
  "${FFMPEG_PREFIX}/lib/libavformat.a" "${FFMPEG_PREFIX}/lib/libavcodec.a"
  "${FFMPEG_PREFIX}/lib/libswscale.a" "${FFMPEG_PREFIX}/lib/libswresample.a"
  "${FFMPEG_PREFIX}/lib/libavutil.a" "${FFMPEG_PREFIX}/lib/zs.lib" bcrypt)
set_target_properties(ffms2 PROPERTIES PREFIX "" OUTPUT_NAME "ffms2")
