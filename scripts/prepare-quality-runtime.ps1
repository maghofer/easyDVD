param([string]$OutputDir, [string]$ScratchDir, [string]$MSVCRoot)
. (Join-Path $PSScriptRoot 'common.ps1')
. (Join-Path $PSScriptRoot 'quality-runtime/common.ps1')
if (-not $OutputDir) { $OutputDir = Get-QualityCacheRoot }
& (Join-Path $PSScriptRoot 'build-quality-runtime.ps1') -OutputDir $OutputDir -ScratchDir $ScratchDir -MSVCRoot $MSVCRoot
