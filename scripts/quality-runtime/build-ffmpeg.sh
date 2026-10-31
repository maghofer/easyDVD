#!/usr/bin/env bash
set -euo pipefail
source_dir=$(cygpath -u "$1")
build_dir=$(cygpath -u "$2")
prefix=$(cygpath -u "$3")
vc_bin=$(cygpath -u "$4")
make_bin=$(cygpath -u "$5")
jobs="$6"
export PATH="$vc_bin:$make_bin:/usr/bin:/bin"
mkdir -p "$build_dir"
cp -R "$source_dir/." "$build_dir/"
cd "$build_dir"
./configure --toolchain=msvc --arch=x86 --prefix=/ \
  --extra-cflags=-MT --disable-everything --disable-autodetect \
  --disable-programs --disable-doc --disable-debug --disable-network \
  --disable-x86asm --enable-static --disable-shared --enable-decoder=ffv1 \
  --enable-demuxer=matroska --enable-protocol=file --enable-swscale --enable-swresample
make -j"$jobs"
make install DESTDIR="$prefix"
