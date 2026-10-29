param([string]$Version, [switch]$IncludeFFmpeg, [switch]$PublicRelease, [string]$CorrespondingSources, [string]$OutputDir)
. (Join-Path $PSScriptRoot 'common.ps1')
$Version = Get-AppVersion $Version
& (Join-Path $PSScriptRoot 'audit-source.ps1')
if (-not $OutputDir) { $OutputDir = Join-Path $ProjectRoot 'dist' }
$OutputDir = [IO.Path]::GetFullPath($OutputDir)
$buildDir = Join-Path ([IO.Path]::GetTempPath()) ('easyDVD-package-' + [guid]::NewGuid().ToString('N'))
if ($IncludeFFmpeg -and $PublicRelease) { throw 'Public offline FFmpeg bundles need a separate complete FFmpeg/dependency source review. Use the normal external-FFmpeg package.' }
$name = "easyDVD-$Version-win64"
if ($IncludeFFmpeg) { $name += '-offline' }
$bundle = Join-Path $OutputDir $name
$sources = "$bundle-sources"
foreach ($path in @($bundle,$sources)) { if (Test-Path -LiteralPath $path) { throw "Package directory already exists: $path" } }
& (Join-Path $PSScriptRoot 'prepare-tools.ps1') -AuthoringOnly:(-not $IncludeFFmpeg)
if (-not $CorrespondingSources) { $CorrespondingSources = Join-Path (Get-AuthoringCacheRoot) 'sources' }
if ($PublicRelease) {
    & (Join-Path $PSScriptRoot 'verify-toolchain-sources.ps1') -Directory $CorrespondingSources -BinaryManifest (Join-Path $ToolsRoot 'media/binaries.json')
}
& (Join-Path $PSScriptRoot 'build.ps1') -Test -IncludeFFmpeg:$IncludeFFmpeg -Version $Version -OutputDir $buildDir
New-Item -ItemType Directory -Force -Path $bundle,(Join-Path $bundle 'tools'),(Join-Path $bundle 'notices'),$sources | Out-Null
foreach ($exe in @('easyDVD.exe','easydvd-cli.exe','easydvd-benchmark.exe')) { Copy-Item -LiteralPath (Join-Path $buildDir $exe) -Destination $bundle }
$manifest = @(Get-Content -LiteralPath (Join-Path $ToolsRoot 'media/binaries.json') -Raw | ConvertFrom-Json | Where-Object { $IncludeFFmpeg -or $_.name -notin @('ffmpeg.exe','ffprobe.exe') })
foreach ($entry in $manifest) { Copy-Item -LiteralPath (Join-Path $ToolsRoot "media/bin/$($entry.name)") -Destination (Join-Path $bundle 'tools') }
ConvertTo-Json -InputObject @($manifest) -Depth 5 | Set-Content -LiteralPath (Join-Path $bundle 'binaries.json') -Encoding UTF8
Copy-Item -LiteralPath (Join-Path (Get-AuthoringCacheRoot) 'sources/notices') -Destination (Join-Path $bundle 'notices/authoring') -Recurse
Copy-Item -LiteralPath (Join-Path $ProjectRoot 'licenses') -Destination (Join-Path $bundle 'notices/build-runtime') -Recurse
Copy-Item -LiteralPath (Join-Path $ProjectRoot 'docs/COWORKER-TEST.md') -Destination (Join-Path $bundle 'START-HERE.md')
if ($IncludeFFmpeg) { foreach ($file in @('FFmpeg-LICENSE.txt','FFmpeg-build.txt')) { Copy-Item -LiteralPath (Join-Path $ToolsRoot "media/notices/$file") -Destination (Join-Path $bundle 'notices') } }
foreach ($file in @('README.md','LICENSE','THIRD_PARTY.md')) { Copy-Item -LiteralPath (Join-Path $ProjectRoot $file) -Destination $bundle; Copy-Item -LiteralPath (Join-Path $ProjectRoot $file) -Destination $sources }
# Copy publishable documentation only; ignored local validation notes stay local.
$documentation = @(& git -C $ProjectRoot ls-files --cached --others --exclude-standard -- docs validation)
Assert-Exit 'List publishable documentation'
foreach ($file in $documentation) {
    $target = Join-Path $bundle $file
    New-Item -ItemType Directory -Force -Path (Split-Path $target) | Out-Null
    Copy-Item -LiteralPath (Join-Path $ProjectRoot $file) -Destination $target
}
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'tools.lock.json') -Destination $bundle
New-Item -ItemType Directory -Force -Path (Join-Path $bundle 'scripts') | Out-Null
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'prepare-hcenc.ps1') -Destination (Join-Path $bundle 'scripts')
$Version | Set-Content -LiteralPath (Join-Path $bundle 'VERSION') -Encoding utf8
@{version=$Version; repository=(Get-Content -LiteralPath (Join-Path $ProjectRoot 'RELEASE_REPOSITORY') -Raw).Trim(); publicRelease=[bool]$PublicRelease; offline=[bool]$IncludeFFmpeg; authoringToolchain=$ToolLock.authoring.id; commit=(& git -C $ProjectRoot rev-parse --verify HEAD 2>$null)} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $bundle 'release.json') -Encoding utf8
Use-DevTools
Push-Location $ProjectRoot
try {
    $moduleGraph = (& go list -m -json all) -join "`n"
    Assert-Exit 'Read module graph'
    @([regex]::Matches($moduleGraph, '(?ms)^\{.*?^\}') | ForEach-Object { $_.Value | ConvertFrom-Json | Select-Object Path,Version,Sum,GoModSum }) | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $bundle 'go-modules.json') -Encoding utf8
    $buildInfo = & go version -m (Join-Path $bundle 'easyDVD.exe')
    Assert-Exit 'Read build info'
    $buildInfo[0] = $buildInfo[0] -replace '^.*: (go[^ ]+)$','easyDVD.exe: $1'
    $buildInfo | Set-Content -LiteralPath (Join-Path $bundle 'build-info.txt') -Encoding utf8
    $moduleSources = Join-Path $sources 'go-modules'
    New-Item -ItemType Directory -Force -Path $moduleSources | Out-Null
    $downloads = & go mod download -json
    Assert-Exit 'Locate dependency sources'
    foreach ($object in [regex]::Matches(($downloads -join "`n"), '(?ms)^\{.*?^\}')) {
        $module = $object.Value | ConvertFrom-Json
        $moduleName = ($module.Path -replace '[/\\]','_') + '@' + $module.Version
        if ($module.Zip) { Copy-Item -LiteralPath $module.Zip -Destination (Join-Path $moduleSources "$moduleName.zip") }
        if ($module.Dir) {
            # Keep notices in the runtime package; large source archives go to the separate bundle.
            $licenseFiles = Get-ChildItem -LiteralPath $module.Dir -File -Recurse | Where-Object { $_.Name -match '^(LICENSE|COPYING|NOTICE|AUTHORS)([._-].*)?$' -or ($module.Path -eq 'golang.org/x/image' -and $_.FullName -match '[\\/]font[\\/]gofont[\\/]ttfs[\\/]README$') }
            foreach ($license in $licenseFiles) {
                $relative = $license.FullName.Substring($module.Dir.Length+1)
                $target = Join-Path $bundle "notices/go/$moduleName/$relative"
                New-Item -ItemType Directory -Force -Path (Split-Path $target) | Out-Null
                Copy-Item -LiteralPath $license.FullName -Destination $target
            }
        }
    }
    $sourceList = Join-Path $buildDir 'package-source-files.txt'
    & git ls-files --cached --others --exclude-standard | Set-Content -LiteralPath $sourceList -Encoding UTF8
    Assert-Exit 'List application source'
    & tar.exe -a -cf (Join-Path $sources 'easyDVD-source.zip') -C $ProjectRoot -T $sourceList
    Assert-Exit 'Archive application source'
    if ($CorrespondingSources) { Copy-Item -LiteralPath $CorrespondingSources -Destination (Join-Path $sources 'toolchain') -Recurse }
    $toolVersions = & (Join-Path $bundle 'easydvd-cli.exe') -check-authoring-tools
    Assert-Exit 'Check bundled authoring tools'
    if ($IncludeFFmpeg) {
        $env:EASYDVD_FFMPEG = Join-Path $bundle 'tools'
        $toolVersions = & (Join-Path $bundle 'easydvd-cli.exe') -check-tools
        Assert-Exit 'Check offline FFmpeg tools'
        Remove-Item Env:EASYDVD_FFMPEG
    }
    $statuses = @(($toolVersions -join "`n") | ConvertFrom-Json)
    foreach ($status in $statuses) { $status.Path = 'tools/' + [IO.Path]::GetFileName($status.Path) }
    ConvertTo-Json -InputObject @($statuses) -Depth 4 | Set-Content -LiteralPath (Join-Path $bundle 'tool-versions.json') -Encoding utf8
} finally { Pop-Location }
foreach ($directory in @($bundle,$sources)) {
    $hashes = Get-ChildItem -LiteralPath $directory -File -Recurse | ForEach-Object { @{path=$_.FullName.Substring($directory.Length+1);sha256=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()} }
    $hashes | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $directory 'SHA256SUMS.json') -Encoding UTF8
    & tar.exe -a -cf "$directory.zip" -C (Split-Path $directory) (Split-Path $directory -Leaf)
    Assert-Exit 'Create package ZIP'
    Write-Output "$directory.zip"
}
