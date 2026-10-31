# Imported after scripts/common.ps1.
$QualityLock = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'lock.json') -Raw | ConvertFrom-Json
function Get-QualityCacheRoot {
    if ($env:EASYDVD_QUALITY_CACHE) { return [IO.Path]::GetFullPath($env:EASYDVD_QUALITY_CACHE) }
    return Join-Path $ToolsRoot "quality-runtime/$($QualityLock.id)"
}
function Assert-QualityRuntime([string]$Directory, [switch]$PublicRelease) {
    $manifestPath = Join-Path $Directory 'binaries.json'
    $manifest = @(Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json)
    if ($manifest.Count -ne $QualityLock.binaries.Count) { throw 'Unexpected Quality support binary count.' }
    foreach ($entry in $QualityLock.binaries) {
        $path = Join-Path $Directory "bin/$($entry.name)"
        $records = @($manifest | Where-Object name -eq $entry.name)
        if ($records.Count -ne 1 -or -not (Test-Path -LiteralPath $path -PathType Leaf) -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $records[0].sha256) {
            throw "Quality runtime binary missing or changed: $($entry.name). Run scripts/prepare-quality-runtime.ps1."
        }
    }
    foreach ($entry in $QualityLock.notices) {
        if (-not (Test-Path -LiteralPath (Join-Path $Directory "notices/$($entry.name)") -PathType Leaf)) { throw "Quality notice missing: $($entry.name)" }
    }
    foreach ($entry in $QualityLock.archives) {
        $path = Join-Path $Directory "sources/archives/$($entry.archive)"
        if (-not (Test-Path -LiteralPath $path -PathType Leaf) -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $entry.sha256) { throw "Quality source/supplier archive missing or changed: $($entry.archive)" }
    }
    & (Join-Path $PSScriptRoot '../verify-toolchain-sources.ps1') -Directory (Join-Path $Directory 'sources') -BinaryManifest $manifestPath
    if ($PublicRelease -and $QualityLock.distributionReview -ne 'complete') { throw "Quality runtime public-distribution review is incomplete: $($QualityLock.reviewNote)" }
}
function Copy-QualityRuntime([string]$Destination) {
    $cache = Get-QualityCacheRoot
    Assert-QualityRuntime $cache
    $runtime = Join-Path $Destination 'tools/quality'
    $notices = Join-Path $Destination 'notices/quality'
    New-Item -ItemType Directory -Force -Path $runtime,$notices | Out-Null
    foreach ($entry in $QualityLock.binaries) { Copy-Item -LiteralPath (Join-Path $cache "bin/$($entry.name)") -Destination $runtime -Force }
    Copy-Item -Path (Join-Path $cache 'notices/*') -Destination $notices -Force
    Copy-Item -LiteralPath (Join-Path $cache 'binaries.json') -Destination (Join-Path $Destination 'quality-binaries.json') -Force
}
