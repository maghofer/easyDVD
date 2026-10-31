# Installs only the build compiler, for the current account; never a dependency
# of easyDVD at runtime. In CI pass a destination under RUNNER_TEMP.
param([Parameter(Mandatory)][string]$Destination)
. (Join-Path $PSScriptRoot 'common.ps1')
$Destination = [IO.Path]::GetFullPath($Destination)
$lock = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'installer/compiler.lock.json') -Raw | ConvertFrom-Json
New-Item -ItemType Directory -Force -Path $Destination | Out-Null
$download = Join-Path $Destination "innosetup-$($lock.version)-x64.exe"
if (-not (Test-Path -LiteralPath $download)) {
    & curl.exe --fail --location --retry 2 --silent --show-error $lock.url --output $download
    Assert-Exit 'Download Inno Setup compiler'
}
if ((Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash -ne $lock.sha256) { throw 'Inno Setup compiler download checksum mismatch.' }
$signature = Get-AuthenticodeSignature -LiteralPath $download
if ($signature.Status -ne 'Valid' -or $signature.SignerCertificate.GetNameInfo([System.Security.Cryptography.X509Certificates.X509NameType]::SimpleName, $false) -ne $lock.publisher) { throw 'Inno Setup compiler signature or publisher is invalid.' }
$compiler = Join-Path $Destination 'compiler/ISCC.exe'
if (-not (Test-Path -LiteralPath $compiler)) {
    $directory = Join-Path $Destination 'compiler'
    $log = Join-Path $Destination 'compiler-install.log'
    $process = Start-Process -FilePath $download -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART','/CURRENTUSER','/NOICONS','/TASKS=""',"/DIR=`"$directory`"", "/LOG=`"$log`"") -WindowStyle Hidden -Wait -PassThru
    if ($process.ExitCode -ne 0) { throw "Compiler installation failed: $($process.ExitCode)" }
}
$compilerVersion = & $compiler --version
Assert-Exit 'Read installed Inno Setup version'
if (($compilerVersion -join "`n").Trim() -ne $lock.version) { throw 'Unexpected installed Inno Setup compiler version.' }
Write-Output $compiler
