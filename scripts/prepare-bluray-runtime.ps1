param([Parameter(Mandatory=$true)][string]$Destination, [Parameter(Mandatory=$true)][string]$Scratch)
$ErrorActionPreference = 'Stop'
$lock = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'bluray/lock.json') -Raw | ConvertFrom-Json
$Destination = [IO.Path]::GetFullPath($Destination)
$Scratch = [IO.Path]::GetFullPath($Scratch)
New-Item -ItemType Directory -Force $Destination,$Scratch,(Join-Path $Scratch 'temp'),(Join-Path $Scratch 'downloads'),(Join-Path $Scratch 'sources') | Out-Null
$env:TEMP = Join-Path $Scratch 'temp'
$env:TMP = $env:TEMP
function Get-Archive($entry) {
    $path = Join-Path $Scratch "downloads/$($entry.archive)"
    if (-not (Test-Path -LiteralPath $path)) { Invoke-WebRequest $entry.url -OutFile $path }
    if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -ne $entry.sha256) { throw "Checksum mismatch: $path" }
    return $path
}
$pythonDir = Join-Path $Destination 'python'
Expand-Archive -LiteralPath (Get-Archive $lock.python) -DestinationPath $pythonDir -Force
$python = Join-Path $pythonDir 'python.exe'
# Explicit module paths only; no user site-packages or installed Python needed.
@('python314.zip','.', '..') | Set-Content -LiteralPath (Join-Path $pythonDir 'python314._pth') -Encoding ascii
foreach ($entry in $lock.packages) {
    $archive = Get-Archive $entry
    & tar -xf $archive -C (Join-Path $Scratch 'sources')
    if ($LASTEXITCODE -ne 0) { throw "Cannot extract $archive" }
    $source = Join-Path $Scratch "sources/$($entry.name)-$($entry.version)"
    $target = Join-Path $Destination $entry.name
    New-Item -ItemType Directory -Force $target | Out-Null
    Copy-Item -Path (Join-Path $source "$($entry.name)/*") -Destination $target -Recurse -Force
    $licenseDir = Join-Path $Destination "licenses/$($entry.name)"
    New-Item -ItemType Directory -Force $licenseDir | Out-Null
    Get-ChildItem -LiteralPath $source -File | Where-Object Name -Match '^(COPYING|LICENSE|NOTICE)' | Copy-Item -Destination $licenseDir
}
& $python (Join-Path $PSScriptRoot 'bluray/patch_bdmvkit.py') (Join-Path $Destination 'bdmvkit')
if ($LASTEXITCODE -ne 0) { throw 'Cannot patch pinned bdmvkit source' }
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'bluray/author.py') -Destination $Destination
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'bluray/patch_bdmvkit.py') -Destination $Destination
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'bluray/lock.json') -Destination $Destination
Write-Output $Destination
