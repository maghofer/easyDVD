param([string]$OutputDir, [string]$CacheDir, [string]$SevenZip)
$ErrorActionPreference = 'Stop'
$setupRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $OutputDir) {
    $OutputDir = if (Test-Path -LiteralPath (Join-Path $setupRoot 'go.mod')) { Join-Path $setupRoot '.tools/hcenc/bin' } else { Join-Path $setupRoot 'tools/hcenc' }
}
if (-not $CacheDir) { $CacheDir = Join-Path ([IO.Path]::GetTempPath()) 'easyDVD-hcenc-setup' }
$OutputDir = [IO.Path]::GetFullPath($OutputDir)
$CacheDir = [IO.Path]::GetFullPath($CacheDir)
if (-not $SevenZip) {
    $found = Get-Command 7z.exe -ErrorAction SilentlyContinue
    if ($found) { $SevenZip = $found.Source }
    elseif (Test-Path -LiteralPath 'C:/Program Files/7-Zip/7z.exe') { $SevenZip = 'C:/Program Files/7-Zip/7z.exe' }
    else { throw 'Install 7-Zip or pass -SevenZip with the path to 7z.exe.' }
}
New-Item -ItemType Directory -Force -Path $CacheDir,$OutputDir | Out-Null
$packages = @(
    @{name='hcenc'; url='http://hank315.nl/files/HC_028/HC028_21-12-2015.zip'; sha256='ed392df990120cd5430454bec68555dd444f1193f930e021c42b1871f5e245a7'},
    @{name='avisynth'; url='https://github.com/AviSynth/AviSynthPlus/releases/download/v3.7.5/AviSynthPlus_3.7.5_20250420-filesonly.7z'; sha256='4db18a1b3787b46ed99e810fa41cdc238f7f6ac82ffa6ed84d53fa8cefb5146f'},
    @{name='ffms2'; url='https://github.com/FFMS/ffms2/releases/download/2.40/ffms2-2.40-msvc.7z'; sha256='0da7454faaab87fc1515d72ffc2edeaee0e6090fa2f27a8986bdebed06197457'}
)
foreach ($package in $packages) {
    $archive = Join-Path $CacheDir ($package.name + [IO.Path]::GetExtension($package.url))
    if (-not (Test-Path -LiteralPath $archive) -or (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $package.sha256) {
        & curl.exe --fail --location --retry 2 --silent --show-error $package.url --output $archive
        if ($LASTEXITCODE -ne 0) { throw "Download failed: $($package.name)" }
    }
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $package.sha256) { throw "Checksum mismatch: $archive" }
    & $SevenZip x $archive "-o$CacheDir/$($package.name)" -y | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Extraction failed: $($package.name)" }
}
$files = @(
    @{source='hcenc/HC028_21-12-2015/HCenc_028.exe'; target='HCenc_028.exe'},
    @{source='hcenc/HC028_21-12-2015/misc/HC028.pdf'; target='HC028.pdf'},
    @{source='avisynth/AviSynthPlus_3.7.5_20250420-filesonly/x86/AviSynth.dll'; target='AviSynth.dll'},
    @{source='avisynth/AviSynthPlus_3.7.5_20250420-filesonly/gpl.txt'; target='AviSynth-GPL.txt'},
    @{source='avisynth/AviSynthPlus_3.7.5_20250420-filesonly/lgpl_for_used_libs.txt'; target='AviSynth-LGPL.txt'},
    @{source='ffms2/ffms2-2.40-msvc/x86/ffms2.dll'; target='ffms2.dll'},
    @{source='ffms2/ffms2-2.40-msvc/COPYING'; target='FFMS2-COPYING.txt'}
)
foreach ($file in $files) { Copy-Item -LiteralPath (Join-Path $CacheDir $file.source) -Destination (Join-Path $OutputDir $file.target) -Force }
$packages | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $OutputDir 'downloads.json') -Encoding UTF8
Write-Host "HCenc runtime ready: $OutputDir"
Write-Host 'Choose this folder in Settings / HCenc folder, or pass -hcenc-dir to the CLI.'
