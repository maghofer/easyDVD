cmake_minimum_required(VERSION 3.20)
project(easyDVD_dvdauthor C)
if(NOT MINGW OR NOT CMAKE_SIZEOF_VOID_P EQUAL 8)
  message(FATAL_ERROR "This recipe targets native Windows x64 with MinGW-w64")
endif()
set(CMAKE_C_STANDARD 17)
set(CMAKE_C_EXTENSIONS ON)
find_package(LibXml2 REQUIRED)
find_package(PNG REQUIRED)
include(CheckSymbolExists)
check_symbol_exists(strndup "string.h" HAVE_STRNDUP)
configure_file(config.h.in config.h)
set(src "${DVDAUTHOR_SOURCE}/src")
include_directories("${CMAKE_CURRENT_BINARY_DIR}" "${src}")
add_compile_definitions(_FILE_OFFSET_BITS=64 LIBXML_STATIC)
add_compile_options(-O2 -ffile-prefix-map=${DVDAUTHOR_SOURCE}=dvdauthor-0.7.2 -ffile-prefix-map=${CMAKE_CURRENT_BINARY_DIR}=build)
add_link_options(-static -static-libgcc -Wl,--no-insert-timestamp)
add_executable(dvdauthor
  ${src}/dvdauthor.c ${src}/dvdcompile.c ${src}/dvdvml.c ${src}/dvdvmy.c
  ${src}/dvdifo.c ${src}/dvdvob.c ${src}/dvdpgc.c ${src}/dvdcli.c
  ${src}/readxml.c ${src}/conffile.c ${src}/compat.c)
target_link_libraries(dvdauthor PRIVATE LibXml2::LibXml2 ws2_32 bcrypt)
add_executable(spumux
  ${src}/subgen.c ${src}/subgen-parse-xml.c ${src}/readxml.c
  ${src}/subgen-encode.c ${src}/subgen-image.c ${src}/conffile.c
  ${src}/compat.c ${src}/subrender.c ${src}/subreader.c ${src}/subfont.c
  image-only.c)
target_link_libraries(spumux PRIVATE LibXml2::LibXml2 PNG::PNG ws2_32 bcrypt)
install(TARGETS dvdauthor spumux RUNTIME DESTINATION bin)
