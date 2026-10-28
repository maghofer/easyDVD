param([Parameter(Mandatory)][string]$OutputDir)
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$entries = Get-Content -LiteralPath (Join-Path $root 'third_party/dvd-tools/source-candidates.json') -Raw | ConvertFrom-Json
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
foreach ($entry in $entries) {
    $target = Join-Path $OutputDir $entry.name
    if (-not (Test-Path -LiteralPath $target) -or (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $entry.sha256) {
        & curl.exe --fail --location --retry 2 --silent --show-error $entry.url --output $target
        if ($LASTEXITCODE -ne 0) { throw "Source download failed: $($entry.name)" }
    }
    if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $entry.sha256) { throw "Source checksum mismatch: $($entry.name)" }
}
Write-Output 'Source candidates acquired and hash-verified. These are not an approved corresponding-source bundle; see validation/THIRD-PARTY-REVIEW.md.'
