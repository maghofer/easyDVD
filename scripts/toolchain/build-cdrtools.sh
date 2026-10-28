#!/usr/bin/env bash
set -euo pipefail
export PATH=/usr/bin
gcc_bin=$(cygpath -u "$1")
source_dir=$(cygpath -u "$2")
jobs=$3
export PATH="$gcc_bin:/usr/bin"
cd "$source_dir"
# Keep legacy C syntax while making pointer-width mistakes hard errors.
flags="-std=gnu89 -fcommon -D_FILE_OFFSET_BITS=64 -Werror=pointer-to-int-cast -Werror=int-to-pointer-cast -ffile-prefix-map=$source_dir=cdrtools-2.01"
for component in libmgw32 inc libschily libdeflt libfile libunls libhfs_iso mkisofs; do
  make -C "$component" -j"$jobs" CCOM=gcc OSNAME=mingw32_nt GMAKE_NOWARN=true \
    COPTX="$flags" LDOPTX="-static -static-libgcc -Wl,--no-insert-timestamp"
done
test -f incs/x86_64-mingw32_nt-gcc/align.h
grep -Eq '^#define[[:space:]]+SIZE_LONG[[:space:]]+4' incs/x86_64-mingw32_nt-gcc/align.h
grep -Eq '^#define[[:space:]]+SIZE_PTR[[:space:]]+8' incs/x86_64-mingw32_nt-gcc/align.h
