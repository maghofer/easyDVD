param([Parameter(Mandatory=$true)][string]$Scratch)
$ErrorActionPreference = 'Stop'
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$Scratch = [IO.Path]::GetFullPath($Scratch)
$downloads = Join-Path $Scratch 'downloads'
$readerRoot = Join-Path $Scratch 'reader'
New-Item -ItemType Directory -Force $downloads,$readerRoot,(Join-Path $Scratch 'temp') | Out-Null
$env:TEMP = Join-Path $Scratch 'temp'
$env:TMP = $env:TEMP
$lock = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'bluray-reader.lock.json') -Raw | ConvertFrom-Json
foreach ($entry in $lock.packages) {
    $archive = Join-Path $downloads $entry.archive
    if (-not (Test-Path -LiteralPath $archive)) { Invoke-WebRequest $entry.url -OutFile $archive }
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $entry.sha256) { throw "Reader archive checksum mismatch: $($entry.archive)" }
    & tar -xf $archive -C $readerRoot
    if ($LASTEXITCODE -ne 0) { throw "Cannot extract $($entry.archive)" }
}
$prefix = Join-Path $readerRoot 'ucrt64'
$output = Join-Path $prefix 'bin/easydvd-bluray-navcheck.exe'
& (Join-Path $repo '.tools/mingw64/bin/gcc.exe') -std=c11 -O2 -I (Join-Path $prefix 'include') (Join-Path $PSScriptRoot 'bluray-navcheck.c') -L (Join-Path $prefix 'lib') -lbluray -o $output
if ($LASTEXITCODE -ne 0) { throw 'Cannot build independent Blu-ray reader' }
Write-Output $output
