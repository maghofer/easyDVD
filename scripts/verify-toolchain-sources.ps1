param([Parameter(Mandatory)][string]$Directory, [Parameter(Mandatory)][string]$BinaryManifest)
$ErrorActionPreference = 'Stop'
$sourceRoot = (Get-Item -LiteralPath $Directory).FullName.TrimEnd('\','/')
$reviewPath = Join-Path $sourceRoot 'review.json'
if (-not (Test-Path -LiteralPath $reviewPath -PathType Leaf)) { throw 'Corresponding sources require review.json mapping every bundled binary to reviewed source, licenses and build instructions.' }
$review = Get-Content -LiteralPath $reviewPath -Raw | ConvertFrom-Json
if ($review.status -ne 'complete') { throw 'Corresponding-source review is not complete.' }
function Require-ReviewFile([string]$Relative) {
    if (-not $Relative -or [IO.Path]::IsPathRooted($Relative)) { throw 'Review file paths must be relative.' }
    $path = [IO.Path]::GetFullPath((Join-Path $sourceRoot $Relative))
    if (-not $path.StartsWith($sourceRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase) -or -not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing or out-of-bounds review file: $Relative" }
    return $path
}
foreach ($file in $review.materialFiles) {
    $path = Require-ReviewFile $file.path
    if (-not $file.sha256 -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $file.sha256) { throw "Source material hash mismatch: $($file.path)" }
}
foreach ($binary in (Get-Content -LiteralPath $BinaryManifest -Raw | ConvertFrom-Json)) {
    $entries = @($review.components | Where-Object { $_.binary -eq $binary.name -and $_.binarySha256 -eq $binary.sha256 })
    if ($entries.Count -ne 1) { throw "Missing or ambiguous source mapping for $($binary.name)." }
    $entry = $entries[0]
    if (-not $entry.sourceArchives -or -not $entry.licenseFiles -or $entry.patchStatus -notin @('none','included')) { throw "Incomplete source/license/patch record for $($binary.name)." }
    foreach ($archive in $entry.sourceArchives) {
        $path = Require-ReviewFile $archive.path
        if (-not $archive.sha256 -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $archive.sha256) { throw "Source archive hash mismatch for $($binary.name)." }
    }
    foreach ($license in $entry.licenseFiles) { $null = Require-ReviewFile $license }
    $null = Require-ReviewFile $entry.buildInstructions
    if ($entry.patchStatus -eq 'included' -and -not $entry.patchFiles) { throw "Missing patches for $($binary.name)." }
    foreach ($patch in $entry.patchFiles) { $null = Require-ReviewFile $patch }
}
Write-Output 'Corresponding-source manifest and all referenced files verified. Legal completeness still depends on the recorded review.'
