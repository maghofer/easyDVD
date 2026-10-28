$ErrorActionPreference = 'Stop'
$ProjectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$ToolsRoot = Join-Path $ProjectRoot '.tools'
$ToolLock = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'tools.lock.json') -Raw | ConvertFrom-Json

function Get-AuthoringCacheRoot {
    if ($env:EASYDVD_AUTHORING_CACHE) { return [IO.Path]::GetFullPath($env:EASYDVD_AUTHORING_CACHE) }
    return Join-Path $ToolsRoot "authoring/$($ToolLock.authoring.id)"
}

function Get-AppVersion([string]$Version) {
    if (-not $Version) { $Version = (Get-Content -LiteralPath (Join-Path $ProjectRoot 'VERSION') -Raw).Trim() }
    if ($Version -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$') { throw 'Version must be SemVer without a v prefix or build metadata, e.g. 0.1.0 or 0.1.0-preview.1.' }
    foreach ($part in (($Version -split '-')[0] -split '\.')) { if ([long]$part -gt 65535) { throw 'Windows version components must be at most 65535.' } }
    if ($Version.Contains('-')) {
        foreach ($part in $Version.Substring($Version.IndexOf('-')+1).Split('.')) {
            if ($part -match '^0[0-9]+$') { throw 'Numeric prerelease identifiers cannot have leading zeros.' }
        }
    }
    return $Version
}

function Get-PinnedArchive($Entry) {
    New-Item -ItemType Directory -Force -Path $ToolsRoot | Out-Null
    $target = Join-Path $ToolsRoot $Entry.archive
    if (-not (Test-Path -LiteralPath $target) -or (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Entry.sha256) {
        & curl.exe --fail --location --retry 2 --silent --show-error $Entry.url --output $target
        if ($LASTEXITCODE -ne 0) { throw "Download failed: $($Entry.url)" }
    }
    if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Entry.sha256) { throw "Checksum mismatch: $target" }
    return $target
}

function Use-DevTools {
    $goBin = Join-Path $ToolsRoot 'go/bin'
    $gccBin = Join-Path $ToolsRoot 'mingw64/bin'
    if (-not (Test-Path -LiteralPath (Join-Path $goBin 'go.exe'))) { throw 'Run scripts/bootstrap.ps1 first.' }
    $env:PATH = "$goBin;$gccBin;$env:PATH"
    $env:CGO_ENABLED = '1'
}

function Assert-Exit($Operation) { if ($LASTEXITCODE -ne 0) { throw "$Operation failed (exit $LASTEXITCODE)" } }
