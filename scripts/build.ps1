param([switch]$Test, [switch]$Integration, [switch]$IncludeFFmpeg, [switch]$AppOnly, [string]$Version, [string]$OutputDir)
. (Join-Path $PSScriptRoot 'common.ps1')
. (Join-Path $PSScriptRoot 'quality-runtime/common.ps1')
$Version = Get-AppVersion $Version
if (-not $OutputDir) { $OutputDir = Join-Path $ProjectRoot 'bin' }
$OutputDir = [IO.Path]::GetFullPath($OutputDir)
$resourceScratch = Join-Path ([IO.Path]::GetTempPath()) ('easyDVD-build-' + [guid]::NewGuid().ToString('N'))
Use-DevTools
if (-not $AppOnly) { & (Join-Path $PSScriptRoot 'prepare-quality-runtime.ps1') }
Push-Location $ProjectRoot
try {
    if ($Test) { & go test ./...; Assert-Exit 'Unit tests' }
    if ($Integration) {
        $oldTools, $oldQuality, $oldIntegration, $oldLocal = $env:EASYDVD_TOOLS, $env:EASYDVD_HCENC, $env:EASYDVD_INTEGRATION, $env:LOCALAPPDATA
        $oldBluray, $oldBlurayIntegration, $oldTemp, $oldTmp = $env:EASYDVD_BLURAY, $env:EASYDVD_BLURAY_INTEGRATION, $env:TEMP, $env:TMP
        try {
            $env:EASYDVD_TOOLS=Join-Path $ToolsRoot 'media/bin'
            $privateRuntime = Join-Path $resourceScratch 'integration-runtime'
            New-Item -ItemType Directory -Force $privateRuntime | Out-Null
            foreach ($entry in $QualityLock.binaries) { Copy-Item -LiteralPath (Join-Path (Get-QualityCacheRoot) "bin/$($entry.name)") -Destination $privateRuntime }
            $encoder = $null
            if ($oldQuality -and (Test-Path -LiteralPath (Join-Path $oldQuality 'HCenc.exe'))) {
                $encoder = Get-Item -LiteralPath (Join-Path $oldQuality 'HCenc.exe')
            } else {
                $env:LOCALAPPDATA=Join-Path $resourceScratch 'profile/local'
                & go run ./cmd/easydvd-cli -download-hcenc -state (Join-Path $resourceScratch 'integration-queue.db')
                Assert-Exit 'Prepare isolated HCEnc test dependency'
                $encoder = Get-ChildItem -LiteralPath (Join-Path $env:LOCALAPPDATA 'easyDVD/hcenc') -Recurse -Filter HCenc.exe | Select-Object -First 1
            }
            if (-not $encoder) { throw 'HCEnc test dependency was not installed.' }
            Copy-Item -LiteralPath $encoder.FullName -Destination (Join-Path $privateRuntime 'HCenc.exe')
            $env:EASYDVD_HCENC=$privateRuntime
            $env:EASYDVD_INTEGRATION='1'
            $env:EASYDVD_BLURAY=Join-Path $resourceScratch 'integration-bluray'
            & (Join-Path $PSScriptRoot 'prepare-bluray-runtime.ps1') -Destination $env:EASYDVD_BLURAY -Scratch (Join-Path $resourceScratch 'integration-bluray-preparation') | Out-Null
            # Keep test media paths short enough for the legacy DVD helpers.
            # Runtime preparation uses its own nested temp directory.
            $env:TEMP=$oldTemp; $env:TMP=$oldTmp
            $env:EASYDVD_BLURAY_INTEGRATION='1'
            & go test -count=1 -v ./internal/integration
            Assert-Exit 'Integration tests'
        } finally {
            $env:EASYDVD_TOOLS=$oldTools; $env:EASYDVD_HCENC=$oldQuality; $env:EASYDVD_INTEGRATION=$oldIntegration; $env:LOCALAPPDATA=$oldLocal
            $env:EASYDVD_BLURAY=$oldBluray; $env:EASYDVD_BLURAY_INTEGRATION=$oldBlurayIntegration; $env:TEMP=$oldTemp; $env:TMP=$oldTmp
        }
    }
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
    if (-not $AppOnly) {
        Copy-QualityRuntime $OutputDir
        $blurayScratch = Join-Path $resourceScratch 'bluray'
        & (Join-Path $PSScriptRoot 'prepare-bluray-runtime.ps1') -Destination (Join-Path $OutputDir 'tools/bluray') -Scratch $blurayScratch
        $bluraySources = Join-Path $OutputDir 'bluray-sources'
        New-Item -ItemType Directory -Force $bluraySources | Out-Null
        Get-ChildItem -LiteralPath (Join-Path $blurayScratch 'downloads') -Filter '*.tar.gz' | Copy-Item -Destination $bluraySources
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'bluray') -Destination (Join-Path $bluraySources 'recipe') -Recurse
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'prepare-bluray-runtime.ps1') -Destination $bluraySources
    }
} finally { Pop-Location }
