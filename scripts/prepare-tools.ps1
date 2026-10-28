param([switch]$AuthoringOnly)
. (Join-Path $PSScriptRoot 'common.ps1')
$authorRoot = Get-AuthoringCacheRoot
& (Join-Path $PSScriptRoot 'build-authoring.ps1') -OutputDir $authorRoot
$target = Join-Path $ToolsRoot 'media/bin'
$notice = Join-Path $ToolsRoot 'media/notices'
New-Item -ItemType Directory -Force -Path $target,$notice | Out-Null

# Prefer an identical already-installed FFmpeg build; otherwise fetch the pinned archive.
if (-not $AuthoringOnly) {
$ffmpegSource = $null
foreach ($candidate in @((Get-Command ffmpeg -ErrorAction SilentlyContinue), (Get-Item -LiteralPath (Join-Path $ToolsRoot 'ffmpeg-9.0.2-full_build/bin/ffmpeg.exe') -ErrorAction SilentlyContinue))) {
    if ($null -eq $candidate) { continue }
    $path = $candidate.Source
    if (-not $path) { $path = $candidate.FullName }
    if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -eq $ToolLock.ffmpeg.ffmpegExeSHA256) { $ffmpegSource = Split-Path $path; break }
}
if (-not $ffmpegSource) {
    $archive = Get-PinnedArchive $ToolLock.ffmpeg
    & tar.exe -xf $archive -C $ToolsRoot
    Assert-Exit 'Extract FFmpeg'
    $ffmpegSource = Join-Path $ToolsRoot 'ffmpeg-9.0.2-full_build/bin'
}
foreach ($name in @('ffmpeg','ffprobe')) {
    $source = Join-Path $ffmpegSource "$name.exe"
    $expected = if ($name -eq 'ffmpeg') { $ToolLock.ffmpeg.ffmpegExeSHA256 } else { $ToolLock.ffmpeg.ffprobeExeSHA256 }
    if ((Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected) { throw "Unexpected $name binary" }
    Copy-Item -LiteralPath $source -Destination $target -Force
}
Copy-Item -LiteralPath (Join-Path (Split-Path $ffmpegSource) 'LICENSE') -Destination (Join-Path $notice 'FFmpeg-LICENSE.txt') -Force
Copy-Item -LiteralPath (Join-Path (Split-Path $ffmpegSource) 'README.txt') -Destination (Join-Path $notice 'FFmpeg-build.txt') -Force
}
Copy-Item -LiteralPath (Join-Path $authorRoot 'sources/notices') -Destination (Join-Path $notice 'authoring') -Recurse -Force
$authoringManifest = @(Get-Content -LiteralPath (Join-Path $authorRoot 'binaries.json') -Raw | ConvertFrom-Json)
foreach ($entry in $authoringManifest) { Copy-Item -LiteralPath (Join-Path $authorRoot "bin/$($entry.name)") -Destination $target -Force }
$names = @($authoringManifest | ForEach-Object name)
if (-not $AuthoringOnly) { $names += @('ffmpeg.exe','ffprobe.exe') }
$files = $names | ForEach-Object { Get-Item -LiteralPath (Join-Path $target $_) } | Sort-Object Name
$manifest = @($files | ForEach-Object { @{ name=$_.Name; sha256=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant() } })
$manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $ToolsRoot 'media/binaries.json') -Encoding UTF8
Write-Output "Prepared pinned tools in $target"
