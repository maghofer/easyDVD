# Exercises real Setup/Uninstall executables, never the user's GUI queue or
# production install registration. Retains fixtures and logs for investigation.
param(
    [Parameter(Mandatory)][string]$InitialBundle,
    [Parameter(Mandatory)][string]$UpgradeBundle,
    [Parameter(Mandatory)][string]$ISCCPath,
    [Parameter(Mandatory)][string]$Scratch,
    [Parameter(Mandatory)][string]$FFmpegDirectory
)
. (Join-Path $PSScriptRoot 'common.ps1')
$Scratch = [IO.Path]::GetFullPath($Scratch)
$run = Join-Path $Scratch ('installer-test-' + [guid]::NewGuid().ToString('N'))
$temp = Join-Path $run 'temp'
New-Item -ItemType Directory -Force -Path $temp | Out-Null
$oldTemp, $oldTmp = $env:TEMP, $env:TMP
$oldLocal = $env:LOCALAPPDATA
$env:TEMP = $temp
$env:TMP = $temp
$env:LOCALAPPDATA = Join-Path $run 'profile/local'
$registry = 'Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Uninstall\easyDVD.InstallerTest_is1'
$menu = Join-Path ([Environment]::GetFolderPath('Programs')) 'easyDVD Installer Test/easyDVD Installer Test.lnk'
$desktop = Join-Path ([Environment]::GetFolderPath('Desktop')) 'easyDVD Installer Test.lnk'
$install = Join-Path $run 'installed app ä'
$report = [System.Collections.Generic.List[string]]::new()
$gui = $null
function Assert-Check([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
    $report.Add($Message)
    Write-Output "PASS: $Message"
}
function Invoke-Setup([string]$Executable, [string[]]$SetupArguments, [bool]$MustSucceed = $true) {
    $process = Start-Process -FilePath $Executable -ArgumentList $SetupArguments -WindowStyle Hidden -PassThru
    if (-not $process.WaitForExit(45000)) { throw "Setup did not finish within 45 seconds: $Executable (PID $($process.Id))" }
    if ($MustSucceed -and $process.ExitCode -ne 0) { throw "Setup failed: $Executable (exit $($process.ExitCode))" }
    return $process.ExitCode
}
function Assert-Payload([string]$Bundle) {
    foreach ($entry in (Get-Content -LiteralPath (Join-Path $Bundle 'SHA256SUMS.json') -Raw | ConvertFrom-Json)) {
        $path = Join-Path $install $entry.path
        if (-not (Test-Path -LiteralPath $path) -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $entry.sha256) { throw "Installed payload mismatch: $($entry.path)" }
    }
}
try {
    if ((Test-Path -LiteralPath $registry) -or (Test-Path -LiteralPath $menu) -or (Test-Path -LiteralPath $desktop)) { throw 'Installer test identity already exists. Inspect and remove the prior test installation first.' }
    $initialVersion = (Get-Content -LiteralPath (Join-Path $InitialBundle 'release.json') -Raw | ConvertFrom-Json).version
    $upgradeVersion = (Get-Content -LiteralPath (Join-Path $UpgradeBundle 'release.json') -Raw | ConvertFrom-Json).version
    if ($initialVersion -eq $upgradeVersion) { throw 'Upgrade validation needs two different application versions.' }
    & (Join-Path $PSScriptRoot 'build-installer.ps1') -BundleDirectory $InitialBundle -OutputDir $run -ISCCPath $ISCCPath -TestIdentity
    & (Join-Path $PSScriptRoot 'build-installer.ps1') -BundleDirectory $UpgradeBundle -OutputDir $run -ISCCPath $ISCCPath -TestIdentity
    $initial = Join-Path $run "easyDVD-$initialVersion-win64-setup-test.exe"
    $upgrade = Join-Path $run "easyDVD-$upgradeVersion-win64-setup-test.exe"
    $silent = @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART','/SP-')
    Invoke-Setup $initial ($silent + @("/DIR=`"$install`"",'/TASKS=desktopicon',"/LOG=`"$run/install.log`"")) | Out-Null
    Assert-Payload $InitialBundle
    Assert-Check ((Test-Path -LiteralPath $menu) -and (Test-Path -LiteralPath $desktop)) 'Start menu and optional desktop shortcuts created'
    $shell = New-Object -ComObject WScript.Shell
    Assert-Check ($shell.CreateShortcut($menu).TargetPath -eq (Join-Path $install 'easyDVD.exe')) 'Shortcut points to the installed application'
    Assert-Check ((Get-ItemProperty -LiteralPath $registry).DisplayVersion -eq $initialVersion) 'Current-user uninstall registration has the initial version'
    Assert-Check ((& (Join-Path $install 'easydvd-cli.exe') -version) -eq "easyDVD $initialVersion") 'Installed CLI reports the initial version'
    Assert-Check (-not (Test-Path -LiteralPath (Join-Path $install 'tools/quality/HCenc.exe'))) 'Installer excludes the separately downloaded HCEnc binary'
    & (Join-Path $install 'easydvd-cli.exe') -download-hcenc -state (Join-Path $run 'queue.db')
    Assert-Exit 'Installed HCEnc download'
    $downloadedEncoder = Get-ChildItem -LiteralPath (Join-Path $env:LOCALAPPDATA 'easyDVD/hcenc') -Recurse -Filter HCenc.exe | Select-Object -First 1
    Assert-Check ($null -ne $downloadedEncoder) 'Installed CLI downloads HCEnc into an isolated private cache'
    $toolJson = & (Join-Path $install 'easydvd-cli.exe') -check-tools -profile Quality -ffmpeg-dir $FFmpegDirectory -state (Join-Path $run 'queue.db')
    Assert-Exit 'Installed Quality/authoring tool check'
    $toolJson | Set-Content -LiteralPath (Join-Path $run 'installed-tools.json') -Encoding utf8
    foreach ($tool in ($toolJson -join "`n" | ConvertFrom-Json)) {
        if ($tool.Name -in @('HCEnc (Quality only)', 'Quality (hcenc) encoder')) {
            if ($tool.Path -ne $downloadedEncoder.FullName) { throw 'Quality did not select the verified private encoder.' }
        } elseif ($tool.Name -notin @('FFmpeg','FFprobe') -and -not $tool.Path.StartsWith($install, [StringComparison]::OrdinalIgnoreCase)) { throw "Tool escaped the installation: $($tool.Name)" }
    }
    Assert-Check $true 'Installed DVD and Quality tools work from the installation directory'
    # Generate source media and a real saved queue through the installed CLI.
    $source = Join-Path $run 'source'
    $work = Join-Path $run 'work'
    $output = Join-Path $run 'output'
    New-Item -ItemType Directory -Force -Path $source,$work,$output | Out-Null
    & (Join-Path $FFmpegDirectory 'ffmpeg.exe') -hide_banner -loglevel error -f lavfi -i 'testsrc2=size=720x576:rate=25' -t 1 -c:v mpeg2video -an (Join-Path $source 'sample.mpg')
    Assert-Exit 'Generate isolated test media'
    $queue = Join-Path $run 'queue.db'
    & (Join-Path $install 'easydvd-cli.exe') -state $queue -work $work -output $output -ffmpeg-dir $FFmpegDirectory -profile Quality -no-menu $source *> (Join-Path $run 'encode.log')
    Assert-Exit 'Installed app encode/author smoke test'
    Assert-Check (@(Get-ChildItem -LiteralPath $output -Filter '*.iso' -Recurse).Count -gt 0) 'Installed CLI authored a DVD ISO using an isolated queue'
    $sentinel = Join-Path $install 'user-added-file.txt'
    'Preserve files not installed by Setup.' | Set-Content -LiteralPath $sentinel
    $preserved = @($queue, $sentinel, $downloadedEncoder.FullName) + @(Get-ChildItem -LiteralPath $source,$work,$output -Recurse -File | Select-Object -ExpandProperty FullName)
    $before = @{}
    foreach ($path in $preserved) { $before[$path] = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash }
    # Launch the actual GUI with an isolated Windows profile; no real queue is read.
    $savedEnv = @{}
    foreach ($name in @('APPDATA','LOCALAPPDATA','USERPROFILE','EASYDVD_FFMPEG','EASYDVD_TOOLS','EASYDVD_HCENC')) { $savedEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
    try {
        $env:APPDATA = Join-Path $run 'profile/roaming'
        $env:LOCALAPPDATA = Join-Path $run 'profile/local'
        $env:USERPROFILE = Join-Path $run 'profile'
        $env:EASYDVD_FFMPEG = $FFmpegDirectory
        $env:EASYDVD_TOOLS = ''
        $env:EASYDVD_HCENC = ''
        $gui = Start-Process -FilePath (Join-Path $install 'easyDVD.exe') -WorkingDirectory $run -PassThru
    } finally {
        foreach ($name in $savedEnv.Keys) { [Environment]::SetEnvironmentVariable($name, $savedEnv[$name], 'Process') }
    }
    $ready = $false
    for ($i=0; $i -lt 30; $i++) {
        Start-Sleep -Milliseconds 300
        $gui.Refresh()
        if ($gui.HasExited) { throw 'Installed GUI exited during startup.' }
        if ($gui.MainWindowTitle -eq "easyDVD $initialVersion") { $ready = $true; break }
    }
    Assert-Check $ready 'Installed GUI opens successfully with an isolated profile'
    $exit = Invoke-Setup $upgrade ($silent + @("/LOG=`"$run/upgrade-blocked.log`"")) $false
    Assert-Check ($exit -ne 0) 'Upgrade refuses to replace a running application'
    Assert-Payload $InitialBundle
    $uninstall = Join-Path $install 'unins000.exe'
    $exit = Invoke-Setup $uninstall @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART',"/LOG=`"$run/uninstall-blocked.log`"") $false
    Assert-Check ($exit -ne 0 -and (Test-Path -LiteralPath (Join-Path $install 'easyDVD.exe'))) 'Uninstall refuses to remove a running application'
    $gui.CloseMainWindow() | Out-Null
    if (-not $gui.WaitForExit(15000)) { throw 'Test GUI did not close normally.' }
    $gui = $null
    # No /DIR: exercise discovery of the previous install location.
    Invoke-Setup $upgrade ($silent + @("/LOG=`"$run/upgrade.log`"")) | Out-Null
    Assert-Payload $UpgradeBundle
    Assert-Check ((Get-ItemProperty -LiteralPath $registry).DisplayVersion -eq $upgradeVersion) 'Upgrade reused the installation location and updated its registration'
    Assert-Check ((& (Join-Path $install 'easydvd-cli.exe') -version) -eq "easyDVD $upgradeVersion") 'Upgraded CLI reports the new version'
    foreach ($path in $preserved) { if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $before[$path]) { throw "Upgrade changed user data: $path" } }
    Assert-Check $true 'Upgrade preserved queue, settings, source, output, work and user-added files'
    # Repeat the same installer as a repair/reinstall.
    Invoke-Setup $upgrade ($silent + @("/LOG=`"$run/reinstall.log`"")) | Out-Null
    Assert-Payload $UpgradeBundle
    Assert-Check $true 'Same-version reinstall succeeds with the same payload'
    Invoke-Setup $uninstall @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART',"/LOG=`"$run/uninstall.log`"") | Out-Null
    Assert-Check (-not (Test-Path -LiteralPath (Join-Path $install 'easyDVD.exe')) -and -not (Test-Path -LiteralPath (Join-Path $install 'tools/quality/avisynth.dll'))) 'Uninstall removed installed application and runtime files'
    Assert-Check (-not (Test-Path -LiteralPath $registry) -and -not (Test-Path -LiteralPath $menu) -and -not (Test-Path -LiteralPath $desktop)) 'Uninstall removed its registration and shortcuts'
    foreach ($path in $preserved) { if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $before[$path]) { throw "Uninstall changed user data: $path" } }
    Assert-Check $true 'Uninstall preserved queue, settings, source, output, work and user-added files'
    $report | Set-Content -LiteralPath (Join-Path $run 'PASS.txt') -Encoding utf8
    Write-Output "Validation artifacts: $run"
} finally {
    if ($gui -and -not $gui.HasExited) { $gui.CloseMainWindow() | Out-Null }
    $env:TEMP = $oldTemp
    $env:TMP = $oldTmp
    $env:LOCALAPPDATA = $oldLocal
}
