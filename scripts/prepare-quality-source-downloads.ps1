# Preseed the original, hash-pinned FFmpeg archive for clean CI builds when its
# origin is unavailable. The normal Quality builder independently verifies it.
param([string]$ScratchDir)
. (Join-Path $PSScriptRoot 'common.ps1')
. (Join-Path $PSScriptRoot 'quality-runtime/common.ps1')
if (-not $ScratchDir) { $ScratchDir = Join-Path ([IO.Path]::GetTempPath()) 'easyDVD-quality-build' }
$source = @($QualityLock.archives | Where-Object archive -like 'ffmpeg-*')
if ($source.Count -ne 1) { throw 'Expected one pinned FFmpeg support source archive.' }
$source = $source[0]
$downloads = Join-Path ([IO.Path]::GetFullPath($ScratchDir)) 'downloads'
New-Item -ItemType Directory -Force -Path $downloads | Out-Null
$archive = Join-Path $downloads $source.archive
if (Test-Path -LiteralPath $archive) {
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne $source.sha256) { throw 'Cached FFmpeg source checksum mismatch.' }
    Write-Output 'Reusing verified FFmpeg support source.'
    return
}
$temporary = Join-Path $downloads ([guid]::NewGuid().ToString('N') + '.download')
try {
    $mirror = 'https://sources.buildroot.net/ffmpeg/' + $source.archive
    & curl.exe --fail --location --silent --show-error --retry 2 --retry-all-errors --retry-max-time 90 --connect-timeout 15 --max-time 60 $mirror --output $temporary
    Assert-Exit 'Download pinned FFmpeg source from Buildroot mirror'
    if ((Get-FileHash -LiteralPath $temporary -Algorithm SHA256).Hash -ne $source.sha256) { throw 'Mirrored FFmpeg source checksum mismatch.' }
    Move-Item -LiteralPath $temporary -Destination $archive
    Write-Output 'Verified original FFmpeg support archive from Buildroot mirror.'
} finally {
    if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Force }
}
