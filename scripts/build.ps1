param([switch]$Test, [switch]$Integration, [switch]$IncludeFFmpeg, [switch]$AppOnly, [string]$Version, [string]$OutputDir)
. (Join-Path $PSScriptRoot 'common.ps1')
$Version = Get-AppVersion $Version
if (-not $OutputDir) { $OutputDir = Join-Path $ProjectRoot 'bin' }
$OutputDir = [IO.Path]::GetFullPath($OutputDir)
$resourceScratch = Join-Path ([IO.Path]::GetTempPath()) ('easyDVD-build-' + [guid]::NewGuid().ToString('N'))
Use-DevTools
Push-Location $ProjectRoot
try {
    if ($Test) { & go test ./...; Assert-Exit 'Unit tests' }
    if ($Integration) { $env:EASYDVD_TOOLS=Join-Path $ToolsRoot 'media/bin'; $env:EASYDVD_INTEGRATION='1'; & go test -count=1 -v ./internal/integration; Assert-Exit 'Integration tests'; Remove-Item Env:EASYDVD_INTEGRATION }
    New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
    $resource = & (Join-Path $PSScriptRoot 'windows-resource.ps1') -Version $Version -Scratch $resourceScratch
    # Stage source so generated Windows resources stay in the temporary build directory.
    $stage = Join-Path $resourceScratch 'source'
    New-Item -ItemType Directory -Force -Path $stage,(Join-Path $stage 'scripts') | Out-Null
    Get-ChildItem -LiteralPath $ProjectRoot -Filter '*.go' -File | Copy-Item -Destination $stage
    foreach ($file in @('go.mod','go.sum','VERSION','RELEASE_REPOSITORY')) { Copy-Item -LiteralPath (Join-Path $ProjectRoot $file) -Destination $stage }
    foreach ($dir in @('internal','cmd')) { Copy-Item -LiteralPath (Join-Path $ProjectRoot $dir) -Destination $stage -Recurse }
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'tools.lock.json') -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath $resource -Destination (Join-Path $stage 'cmd/easydvd/resource_windows_amd64.syso')
    & go -C $stage build -trimpath -ldflags "-s -w -H windowsgui -X easydvd.BuildVersion=$Version" -o (Join-Path $OutputDir 'easyDVD.exe') ./cmd/easydvd
    Assert-Exit 'Build desktop app'
    & go -C $stage build -trimpath -ldflags "-s -w -X easydvd.BuildVersion=$Version" -o (Join-Path $OutputDir 'easydvd-cli.exe') ./cmd/easydvd-cli
    Assert-Exit 'Build CLI'
    & go -C $stage build -trimpath -ldflags "-s -w -X easydvd.BuildVersion=$Version" -o (Join-Path $OutputDir 'easydvd-benchmark.exe') ./cmd/easydvd-benchmark
    Assert-Exit 'Build benchmark'
    $manifestPath = Join-Path $ToolsRoot 'media/binaries.json'
    if (-not $AppOnly -and (Test-Path -LiteralPath $manifestPath)) {
        $runtimeDir = Join-Path $OutputDir 'tools'
        New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
        foreach ($entry in (Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json)) {
            if (-not $IncludeFFmpeg -and $entry.name -in @('ffmpeg.exe','ffprobe.exe')) { continue }
            Copy-Item -LiteralPath (Join-Path $ToolsRoot "media/bin/$($entry.name)") -Destination $runtimeDir -Force
        }
        if (-not $IncludeFFmpeg) {
            foreach ($name in @('ffmpeg.exe','ffprobe.exe')) {
                $path = Join-Path $runtimeDir $name
                $expected = if ($name -eq 'ffmpeg.exe') { $ToolLock.ffmpeg.ffmpegExeSHA256 } else { $ToolLock.ffmpeg.ffprobeExeSHA256 }
                if ((Test-Path -LiteralPath $path) -and (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -eq $expected) { Remove-Item -LiteralPath $path -Force }
            }
        }
    }
} finally { Pop-Location }
