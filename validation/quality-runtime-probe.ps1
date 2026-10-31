# Run with Windows' 32-bit PowerShell. No registry or system files are changed.
param([Parameter(Mandatory)][string]$RuntimePath)
$ErrorActionPreference = 'Stop'
if ([IntPtr]::Size -ne 4) { throw 'Use 32-bit PowerShell for this probe.' }
$RuntimePath = (Resolve-Path -LiteralPath $RuntimePath).Path
Add-Type @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class QualityRuntimeProbe {
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
    public static extern IntPtr LoadLibraryEx(string path, IntPtr reserved, uint flags);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode)]
    public static extern IntPtr GetModuleHandle(string name);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode)]
    public static extern uint GetModuleFileName(IntPtr module, StringBuilder path, int size);
}
'@
foreach ($name in @('avisynth.dll','ffms2.dll')) {
    # LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR | LOAD_LIBRARY_SEARCH_SYSTEM32:
    # exclude PATH, the current directory and installed application folders.
    $module = [QualityRuntimeProbe]::LoadLibraryEx((Join-Path $RuntimePath $name), [IntPtr]::Zero, 0x900)
    if ($module -eq [IntPtr]::Zero) { throw "Cannot load bundled $name; Windows error $([Runtime.InteropServices.Marshal]::GetLastWin32Error())." }
}
$names = @('avisynth.dll','ffms2.dll')
# Classic custom runtimes import DevIL; the source-built distribution does not.
if ([QualityRuntimeProbe]::GetModuleHandle('DevIL.dll') -ne [IntPtr]::Zero) { $names += 'DevIL.dll' }
$result = foreach ($name in $names) {
    $module = [QualityRuntimeProbe]::GetModuleHandle($name)
    if ($module -eq [IntPtr]::Zero) { throw "$name was not loaded." }
    $path = New-Object Text.StringBuilder 32768
    $null = [QualityRuntimeProbe]::GetModuleFileName($module, $path, $path.Capacity)
    if (-not $path.ToString().Equals((Join-Path $RuntimePath $name), [StringComparison]::OrdinalIgnoreCase)) { throw "$name was loaded from outside the selected runtime: $path" }
    @{name=$name;path=$path.ToString()}
}
$result | ConvertTo-Json
