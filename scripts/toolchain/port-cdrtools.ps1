# Source patch for cdrtools 2.01-bootcd.ru on native MinGW-w64/UCRT x64.
# Apply only to a freshly extracted, hash-verified source archive.
param([Parameter(Mandatory)][string]$Source)
$ErrorActionPreference = 'Stop'
function Rewrite([string]$Relative, [scriptblock]$Transform) {
    $path = Join-Path $Source $Relative
    $text = Get-Content -LiteralPath $path -Raw
    [IO.File]::WriteAllText($path, (& $Transform $text))
}
# Avoid colliding with the compiler's own CRT vadefs.h.
Move-Item -LiteralPath (Join-Path $Source 'include/vadefs.h') -Destination (Join-Path $Source 'include/schily_vadefs.h')
foreach ($file in Get-ChildItem -LiteralPath $Source -Recurse -File | Where-Object { $_.Extension -in @('.c','.h') }) {
    $text = Get-Content -LiteralPath $file.FullName -Raw
    if ($text.Contains('<vadefs.h>')) { [IO.File]::WriteAllText($file.FullName, $text.Replace('<vadefs.h>', '<schily_vadefs.h>')) }
}
Copy-Item -LiteralPath (Join-Path $Source 'RULES/i386-mingw32_nt-gcc.rul') -Destination (Join-Path $Source 'RULES/x86_64-mingw32_nt-gcc.rul')
Rewrite 'include/mconfig.h' {
    param($s)
    $s = $s.Replace('#ifdef __cplusplus', "#ifdef __MINGW32__`n#undef u_char`n#undef u_short`n#undef u_int`n#undef u_long`n#include <winsock2.h>`n#include <stdint.h>`n#endif`n`n#ifdef __cplusplus")
    $s = [regex]::Replace($s, '#define\s+sleep\(a\)[^\n]+\n', '')
    $s = [regex]::Replace($s, '/\* Mingw32 has no timezone structure \*/.*?#define HAVE_GETTIMEOFDAY 1', '#define HAVE_GETTIMEOFDAY 1', 'Singleline')
    # Modern MinGW-w64 supplies 64-bit time functions; the old 2038 workaround is unnecessary.
    $s = [regex]::Replace($s, '#define\s+(gmtime|localtime)\(a\)[^\n]+\n', '')
    return $s
}
Rewrite 'libmgw32/mgw32.c' {
    param($s)
    $s = $s.Replace('#include <windows.h>', "#include <windows.h>`n#include <stdio.h>")
    return [regex]::Replace($s, 'struct\s+timezone\s*\{.*?(?=int __mgw32__stat)', '', 'Singleline')
}
Rewrite 'include/standard.h' { param($s) [regex]::Replace($s, 'typedef\s+void\s+VOID;', "#ifndef VOID`n`ttypedef void VOID;`n#endif") }
# easyDVD masters files into a new ISO. It does not access physical drives/ASPI.
Rewrite 'mkisofs/Makefile' {
    param($s)
    $s = $s.Replace('-DUSE_SCG','').Replace('-lrscg -lscg','')
    $s = [regex]::Replace($s, '(?m)^\s*(?:scsi_cdr\.c cd_misc\.c|modes\.c) \\\r?\n', '')
    return $s.Replace('defaults.c getnum.c md5c.c', 'md5c.c')
}
Rewrite 'mkisofs/multi.c' { param($s) [regex]::Replace($s, 'LOCAL int(\r?\nreadsecs)', 'EXPORT int$1') }
Rewrite 'mkisofs/scsi.h' {
    param($s)
    $s.Replace("#ifdef`tUSE_SCG", "extern int readsecs __PR((int startsecno, void *buffer, int sectorcount));`n#ifdef`tUSE_SCG")
}
Rewrite 'mkisofs/mkisofs.c' { param($s) $s.Replace('mkisofs 2.01-bootcd.ru', 'mkisofs 2.01-bootcd.ru-easyDVD.1') }
# Windows uses LLP64: pointers are 64-bit but long is 32-bit. Opaque printf
# callback/condition arguments must be pointer-sized, without changing numeric long formatting.
foreach ($name in @('jsprintf','jssnprintf','jssprintf','printf','snprintf','sprintf')) {
    Rewrite "libschily/$name.c" { param($s) [regex]::Replace($s, '\blong\b', 'intptr_t') }
}
Rewrite 'libschily/format.c' {
    param($s)
    $s = $s.Replace('char, long', 'char, intptr_t')
    return [regex]::Replace($s, '\blong(\s+)(farg|arg)\b', 'intptr_t$1$2')
}
foreach ($file in @('include/sigblk.h','libschily/handlecond.c','libschily/raisecond.c')) {
    Rewrite $file { param($s) [regex]::Replace($s, '\blong\b', 'intptr_t') }
}
Rewrite 'include/schily.h' {
    param($s)
    $s = $s.Replace('(void (*)(char, long), long,', '(void (*)(char, intptr_t), intptr_t,')
    return [regex]::Replace($s, '(raisecond\s+__PR\(\(const char \*, )long(\)\))', '$1intptr_t$2')
}
Rewrite 'libschily/getargs.c' { param($s) $s.Replace('(long)fmt', '(intptr_t)fmt') }
Rewrite 'inc/align_test.c' {
    param($s)
    $s = $s.Replace('((int)&((s)0)->m)', '((int)(intptr_t)&((s)0)->m)')
    return $s.Replace('((int)buf) % 1024', '((uintptr_t)buf) % 1024')
}
