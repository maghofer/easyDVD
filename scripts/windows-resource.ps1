param([Parameter(Mandatory)][string]$Version, [Parameter(Mandatory)][string]$Scratch)
. (Join-Path $PSScriptRoot 'common.ps1')
Use-DevTools
New-Item -ItemType Directory -Force -Path $Scratch | Out-Null
$icon = (Join-Path $ProjectRoot 'internal/ui/assets/app.ico').Replace('\','/')
$parts = ($Version -split '[-+]')[0].Split('.')
$numericVersion = ($parts + @('0')) -join ','
$rc = Join-Path $Scratch 'app.rc'
@"
#include <windows.h>
1 ICON "$icon"
1 VERSIONINFO
FILEVERSION $numericVersion
PRODUCTVERSION $numericVersion
FILEOS VOS_NT_WINDOWS32
FILETYPE VFT_APP
BEGIN
  BLOCK "StringFileInfo"
  BEGIN
    BLOCK "040904b0"
    BEGIN
      VALUE "FileDescription", "easyDVD"
      VALUE "FileVersion", "$Version"
      VALUE "ProductName", "easyDVD"
      VALUE "ProductVersion", "$Version"
      VALUE "OriginalFilename", "easyDVD.exe"
    END
  END
  BLOCK "VarFileInfo"
  BEGIN
    VALUE "Translation", 0x0409, 1200
  END
END
"@ | Set-Content -LiteralPath $rc -Encoding utf8
$resource = Join-Path $Scratch 'resource_windows_amd64.syso'
& windres --input $rc --output $resource --output-format coff --target pe-x86-64
Assert-Exit 'Compile Windows icon and version resource'
return $resource
