param([string]$ScratchDir, [string]$OutputDir, [string]$MSVCRoot, [int]$Jobs=6)
. (Join-Path $PSScriptRoot 'common.ps1')
. (Join-Path $PSScriptRoot 'quality-runtime/common.ps1')
if (-not $OutputDir) { $OutputDir = Get-QualityCacheRoot }
if (-not $ScratchDir) { $ScratchDir = Join-Path ([IO.Path]::GetTempPath()) 'easyDVD-quality-build' }
$OutputDir = [IO.Path]::GetFullPath($OutputDir)
$ScratchDir = [IO.Path]::GetFullPath($ScratchDir)
if ($Jobs -lt 1 -or $Jobs -gt 32) { throw 'Jobs must be between 1 and 32.' }
$recipeRoot = Join-Path $PSScriptRoot 'quality-runtime'
$recipeFiles = @((Get-Item -LiteralPath $PSCommandPath)) + @(Get-ChildItem -LiteralPath $recipeRoot -File | Where-Object Name -ne 'README.md')
$recipeText = ($recipeFiles | Sort-Object FullName | ForEach-Object { $_.Name + ':' + (Get-FileHash $_.FullName -Algorithm SHA256).Hash }) -join "`n"
$key = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($recipeText))).ToLowerInvariant()
try {
    Assert-QualityRuntime $OutputDir
    $record = Get-Content -LiteralPath (Join-Path $OutputDir 'build-key.json') -Raw | ConvertFrom-Json
    if ($record.recipeKey -ne $key) { throw 'Quality build recipe changed.' }
    Write-Output "Reusing source-built Quality support ($($QualityLock.id))."
    return
} catch { }
if ((Test-Path -LiteralPath $OutputDir) -and @(Get-ChildItem -LiteralPath $OutputDir -Force).Count) { throw 'Quality output is nonempty and cannot be replaced. Use a new runtime ID/output directory for a changed recipe.' }
$oldEnv = @{}
foreach ($name in @('TEMP','TMP','TMPDIR','PATH','INCLUDE','LIB','VCToolsInstallDir','WindowsSdkDir')) { $oldEnv[$name] = [Environment]::GetEnvironmentVariable($name,'Process') }
function Fetch-Quality($entry) {
    $path = Join-Path $downloads $entry.archive
    if (-not (Test-Path -LiteralPath $path)) {
        & curl.exe -fLsS --retry 2 $entry.url -o $path
        Assert-Exit "Download $($entry.archive)"
    }
    if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $entry.sha256) { throw "Quality input checksum mismatch: $($entry.archive)" }
    return $path
}
function Run-Quality([string]$Name, [string]$Exe, [string[]]$Arguments) {
    & $Exe @Arguments *> (Join-Path $work "$Name.log")
    if ($LASTEXITCODE -ne 0) { Get-Content (Join-Path $work "$Name.log") -Tail 25 | Write-Host; throw "Quality build failed: $Name" }
}
try {
    $downloads = Join-Path $ScratchDir 'downloads'
    $work = Join-Path $ScratchDir ('build-' + [guid]::NewGuid().ToString('N'))
    $temp = Join-Path $ScratchDir 'temp'
    $src = Join-Path $work 'src'
    $prefix = Join-Path $work 'prefix'
    New-Item -ItemType Directory -Force -Path $downloads,$work,$temp,$src | Out-Null
    $env:TEMP=$temp; $env:TMP=$temp; $env:TMPDIR=$temp.Replace('\','/')
    if ($MSVCRoot) {
        $vc = (Get-ChildItem -LiteralPath (Join-Path $MSVCRoot 'VC/Tools/MSVC') -Directory | Sort-Object Name -Descending | Select-Object -First 1).FullName
        $sdk = Join-Path $MSVCRoot 'Windows Kits/10'
    } else {
        $vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio/Installer/vswhere.exe'
        if (-not (Test-Path $vswhere)) { throw 'MSVC C++ build tools are required. Install the C++ workload, or pass -MSVCRoot with a portable VC/Windows Kits tree.' }
        $vs = & $vswhere -latest -products '*' -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
        if (-not $vs) { throw 'No Visual Studio C++ toolset found.' }
        $vc = (Get-ChildItem -LiteralPath (Join-Path $vs 'VC/Tools/MSVC') -Directory | Sort-Object Name -Descending | Select-Object -First 1).FullName
        $sdk = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits/10'
    }
    $sdkVersion = (Get-ChildItem -LiteralPath (Join-Path $sdk 'Include') -Directory | Where-Object { Test-Path (Join-Path $_.FullName 'um/Windows.h') } | Sort-Object Name -Descending | Select-Object -First 1).Name
    $vcBin = Join-Path $vc 'bin/Hostx64/x86'
    $env:PATH = "$vcBin;$sdk/bin/$sdkVersion/x64;$env:PATH"
    $env:INCLUDE = "$vc/include;$sdk/Include/$sdkVersion/ucrt;$sdk/Include/$sdkVersion/shared;$sdk/Include/$sdkVersion/um"
    $env:LIB = "$vc/lib/x86;$sdk/Lib/$sdkVersion/ucrt/x86;$sdk/Lib/$sdkVersion/um/x86"
    $env:VCToolsInstallDir="$vc/"; $env:WindowsSdkDir="$sdk/"
    $cmake = Join-Path $ToolsRoot 'mingw64/bin/cmake.exe'
    $ninja = Join-Path $ToolsRoot 'mingw64/bin/ninja.exe'
    if (-not (Test-Path $cmake) -or -not (Test-Path $ninja)) { throw 'Run scripts/bootstrap.ps1 to prepare CMake/Ninja.' }
    $env:PATH = "$(Split-Path $ninja);$env:PATH"
    $git = (Get-Command git.exe -ErrorAction Stop).Source
    $bash = [IO.Path]::GetFullPath((Join-Path (Split-Path $git) '../bin/bash.exe'))
    if (-not (Test-Path $bash)) { throw 'Git for Windows with Bash is required.' }
    foreach ($entry in $QualityLock.archives) {
        $archive = Fetch-Quality $entry
        & tar.exe -xf $archive -C $src
        Assert-Exit "Extract $($entry.archive)"
    }
    $makeArchive = Fetch-Quality $QualityLock.make
    $makeRoot = Join-Path $work 'make'
    New-Item -ItemType Directory -Force $makeRoot | Out-Null
    & tar.exe -xf $makeArchive -C $makeRoot
    Assert-Exit 'Extract GNU make'
    $avs = Join-Path $src 'AviSynthPlus-3.7.6'
    $ffms = Join-Path $src 'ffms2-5.0'
    Run-Quality 'avisynth-configure' $cmake @('-S',$avs,'-B',"$work/avisynth",'-G','Ninja','-DCMAKE_CXX_COMPILER=cl.exe','-DCMAKE_BUILD_TYPE=Release','-DCMAKE_CXX_FLAGS_RELEASE=/MT /O2 /Ob2 /DNDEBUG','-DENABLE_PLUGINS=OFF','-DENABLE_CUDA=OFF')
    Run-Quality 'avisynth-build' $cmake @('--build',"$work/avisynth",'--parallel',"$Jobs")
    Run-Quality 'ffmpeg' $bash @('--noprofile','--norc',(Join-Path $recipeRoot 'build-ffmpeg.sh'),(Join-Path $src 'ffmpeg-6.1.3'),"$work/ffmpeg",$prefix,$vcBin,(Join-Path $makeRoot 'usr/bin'),"$Jobs")
    Run-Quality 'zlib-configure' $cmake @('-S',"$src/zlib-1.3.2",'-B',"$work/zlib",'-G','Ninja','-DCMAKE_C_COMPILER=cl.exe','-DCMAKE_BUILD_TYPE=Release','-DCMAKE_MSVC_RUNTIME_LIBRARY=MultiThreaded',"-DCMAKE_INSTALL_PREFIX=$prefix",'-DZLIB_BUILD_SHARED=OFF','-DZLIB_BUILD_STATIC=ON','-DZLIB_BUILD_TESTING=OFF')
    Run-Quality 'zlib-build' $cmake @('--build',"$work/zlib",'--parallel',"$Jobs")
    Run-Quality 'zlib-install' $cmake @('--install',"$work/zlib")
    Run-Quality 'ffms-configure' $cmake @('-S',$recipeRoot,'-B',"$work/ffms",'-G','Ninja','-DCMAKE_C_COMPILER=cl.exe','-DCMAKE_CXX_COMPILER=cl.exe','-DCMAKE_BUILD_TYPE=Release','-DCMAKE_MSVC_RUNTIME_LIBRARY=MultiThreaded',"-DFFMS_SOURCE=$ffms","-DAVS_SOURCE=$avs","-DFFMPEG_PREFIX=$prefix")
    Run-Quality 'ffms-build' $cmake @('--build',"$work/ffms",'--parallel',"$Jobs")
    $material = Join-Path $work 'material'
    New-Item -ItemType Directory -Force -Path "$material/archives","$material/notices","$material/recipe/scripts" | Out-Null
    foreach ($entry in $QualityLock.archives) { Copy-Item (Join-Path $downloads $entry.archive) (Join-Path $material "archives/$($entry.archive)") }
    foreach ($name in @('build-quality-runtime.ps1','common.ps1','tools.lock.json','verify-toolchain-sources.ps1')) { Copy-Item (Join-Path $PSScriptRoot $name) (Join-Path $material 'recipe/scripts') }
    Copy-Item $recipeRoot (Join-Path $material 'recipe/scripts/quality-runtime') -Recurse
    Copy-Item (Join-Path $recipeRoot 'README.md') (Join-Path $material 'BUILD.md')
    $noticeFiles = @{
        'AviSynth-GPL-2.0.txt'="$avs/distrib/gpl.txt";
        'AviSynth-license.rst'="$avs/distrib/docs/english/source/avisynthdoc/license.rst";
        'FFMS2-MIT.txt'="$ffms/COPYING";
        'FFmpeg-LGPL-2.1.txt'="$src/ffmpeg-6.1.3/COPYING.LGPLv2.1";
        'zlib-LICENSE.txt'="$src/zlib-1.3.2/LICENSE"
    }
    foreach ($entry in $noticeFiles.GetEnumerator()) { Copy-Item -LiteralPath $entry.Value -Destination (Join-Path $material "notices/$($entry.Key)") }
    $manifest = @()
    foreach ($entry in @(@{name='avisynth.dll';path="$work/avisynth/avs_core/AviSynth.dll"},@{name='ffms2.dll';path="$work/ffms/ffms2.dll"})) {
        $imports = & (Join-Path $vcBin 'dumpbin.exe') /dependents $entry.path
        Assert-Exit 'Inspect Quality DLL dependencies'
        $dlls = @($imports | ForEach-Object { if ($_ -match '^\s+([a-zA-Z0-9_.-]+\.dll)\s*$') { $Matches[1] } })
        $unexpected = @($dlls | Where-Object { $_ -notmatch '^(?i:kernel32|user32|advapi32|gdi32|ole32|imagehlp|avifil32|msvfw32|msacm32|winmm|bcrypt)\.dll$' })
        if ($unexpected.Count) { throw "Unreviewed runtime DLL dependencies: $($unexpected -join ', ')" }
        $manifest += [ordered]@{name=$entry.name;sha256=(Get-FileHash $entry.path -Algorithm SHA256).Hash.ToLowerInvariant();imports=$dlls}
    }
    $archiveRecords = @($QualityLock.archives | ForEach-Object { @{path='archives/'+$_.archive;sha256=$_.sha256} })
    $components = @($manifest | ForEach-Object { [ordered]@{binary=$_.name;binarySha256=$_.sha256;sourceArchives=$archiveRecords;licenseFiles=@($noticeFiles.Keys | ForEach-Object { 'notices/'+$_ });buildInstructions='BUILD.md';patchStatus='included';patchFiles=@('recipe/scripts/quality-runtime/CMakeLists.txt','recipe/scripts/quality-runtime/build-ffmpeg.sh')} })
    $files = @(Get-ChildItem $material -Recurse -File | ForEach-Object { @{path=[IO.Path]::GetRelativePath($material,$_.FullName).Replace('\','/');sha256=(Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()} })
    [ordered]@{status='complete';toolchain=$QualityLock.id;recipeKey=$key;compiler=(Get-Item (Join-Path $vcBin 'cl.exe')).VersionInfo.FileVersion;sdk=$sdkVersion;components=$components;materialFiles=$files} | ConvertTo-Json -Depth 9 | Set-Content "$material/review.json" -Encoding utf8
    $tempManifest = Join-Path $work 'binaries.json'
    ConvertTo-Json -InputObject @($manifest) -Depth 5 | Set-Content $tempManifest -Encoding utf8
    & (Join-Path $PSScriptRoot 'verify-toolchain-sources.ps1') -Directory $material -BinaryManifest $tempManifest
    New-Item -ItemType Directory -Force -Path "$OutputDir/bin","$OutputDir/notices" | Out-Null
    Copy-Item "$work/avisynth/avs_core/AviSynth.dll" "$OutputDir/bin/avisynth.dll"
    Copy-Item "$work/ffms/ffms2.dll" "$OutputDir/bin/ffms2.dll"
    Copy-Item "$material/notices/*" "$OutputDir/notices"
    Copy-Item $material "$OutputDir/sources" -Recurse
    Copy-Item $tempManifest "$OutputDir/binaries.json"
    @{recipeKey=$key} | ConvertTo-Json | Set-Content "$OutputDir/build-key.json" -Encoding utf8
    Assert-QualityRuntime $OutputDir -PublicRelease
    Write-Output "Built source-recorded Quality support ($($QualityLock.id))."
} finally {
    foreach ($name in $oldEnv.Keys) { [Environment]::SetEnvironmentVariable($name,$oldEnv[$name],'Process') }
}
