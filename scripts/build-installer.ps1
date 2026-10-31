param(
    [Parameter(Mandatory)][string]$BundleDirectory,
    [string]$OutputDir,
    [string]$ISCCPath,
    [switch]$PublicRelease,
    [switch]$TestIdentity
)
. (Join-Path $PSScriptRoot 'common.ps1')
$BundleDirectory = (Resolve-Path -LiteralPath $BundleDirectory).Path
if (-not $OutputDir) { $OutputDir = Join-Path $ProjectRoot 'dist' }
$OutputDir = [IO.Path]::GetFullPath($OutputDir)
if ($OutputDir.TrimEnd('\') -eq $BundleDirectory.TrimEnd('\') -or $OutputDir.StartsWith($BundleDirectory.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Installer output must be outside the runtime package directory.' }
if ($TestIdentity -and $PublicRelease) { throw 'A test-identity installer cannot be a public release.' }
$metadata = Get-Content -LiteralPath (Join-Path $BundleDirectory 'release.json') -Raw | ConvertFrom-Json
$version = Get-AppVersion $metadata.version
if ($PublicRelease -and $metadata.publicRelease -ne $true) { throw 'Public installers require a runtime package built with -PublicRelease. Distribution reviews must pass first.' }
foreach ($name in @('easyDVD.exe','easydvd-cli.exe','easydvd-benchmark.exe','LICENSE','THIRD_PARTY.md','tools/dvdauthor.exe','tools/spumux.exe','tools/mkisofs.exe','quality-binaries.json')) {
    if (-not (Test-Path -LiteralPath (Join-Path $BundleDirectory $name) -PathType Leaf)) { throw "Runtime package is incomplete: $name" }
}
if ((Get-Item -LiteralPath (Join-Path $BundleDirectory 'easyDVD.exe')).VersionInfo.ProductVersion -ne $version) { throw 'Application version does not match release.json.' }
# Never compile a wildcard payload until every included file is accounted for.
$hashes = @(Get-Content -LiteralPath (Join-Path $BundleDirectory 'SHA256SUMS.json') -Raw | ConvertFrom-Json)
foreach ($item in (Get-ChildItem -LiteralPath $BundleDirectory -Recurse -Force)) {
    if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Package files and directories cannot be links.' }
}
$expected = @{}
foreach ($entry in $hashes) {
    $path = [IO.Path]::GetFullPath((Join-Path $BundleDirectory $entry.path))
    if (-not $path.StartsWith($BundleDirectory.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) { throw "Package manifest escapes its directory: $($entry.path)" }
    if ($expected.ContainsKey($path)) { throw "Duplicate package manifest entry: $($entry.path)" }
    if ($entry.sha256 -notmatch '^[0-9a-fA-F]{64}$' -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $entry.sha256) { throw "Package checksum mismatch: $($entry.path)" }
    $expected[$path] = $true
}
foreach ($file in (Get-ChildItem -LiteralPath $BundleDirectory -Recurse -File -Force)) {
    if ($file.FullName -ne (Join-Path $BundleDirectory 'SHA256SUMS.json') -and -not $expected.ContainsKey($file.FullName)) { throw "Unlisted file in runtime package: $($file.FullName)" }
    if ($file.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Package files cannot be links.' }
}
if (-not $ISCCPath) {
    $command = Get-Command ISCC.exe -ErrorAction SilentlyContinue
    if ($command) { $ISCCPath = $command.Source }
    foreach ($base in @($env:ProgramFiles, ${env:ProgramFiles(x86)}, (Join-Path $env:LOCALAPPDATA 'Programs'))) {
        foreach ($major in @('7','6')) {
            $candidate = Join-Path $base "Inno Setup $major/ISCC.exe"
            if (-not $ISCCPath -and (Test-Path -LiteralPath $candidate)) { $ISCCPath = $candidate }
        }
    }
}
if (-not $ISCCPath -or -not (Test-Path -LiteralPath $ISCCPath -PathType Leaf)) { throw 'Install Inno Setup 7, put ISCC.exe on PATH, or pass -ISCCPath.' }
$suffix = if ($TestIdentity) { '-test' } else { '' }
$installer = Join-Path $OutputDir "easyDVD-$version-win64-setup$suffix.exe"
if (Test-Path -LiteralPath $installer) { throw "Installer already exists: $installer" }
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$arguments = @('/Qp', "/DBundleDir=$BundleDirectory", "/DAppVersion=$version", "/DInstallerOutputDir=$OutputDir")
if ($TestIdentity) { $arguments += '/DInstallerTest=1' }
& $ISCCPath @arguments (Join-Path $PSScriptRoot 'installer/easyDVD.iss')
Assert-Exit 'Build Windows installer'
if (-not (Test-Path -LiteralPath $installer)) { throw 'Compiler did not produce the installer.' }
$hash = (Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash.ToLowerInvariant()
"$hash  $([IO.Path]::GetFileName($installer))" | Set-Content -LiteralPath "$installer.sha256" -Encoding utf8
Write-Output $installer
