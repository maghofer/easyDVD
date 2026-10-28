param([string]$ScratchDir, [string]$OutputDir, [int]$Jobs=6, [switch]$Force)
. (Join-Path $PSScriptRoot 'common.ps1')
$recipeRoot = Join-Path $PSScriptRoot 'toolchain'
$lock = Get-Content -LiteralPath (Join-Path $recipeRoot 'lock.json') -Raw | ConvertFrom-Json
if (-not $ScratchDir) { $ScratchDir = Join-Path ([IO.Path]::GetTempPath()) 'easyDVD-authoring' }
if (-not $OutputDir) { $OutputDir = Join-Path $ToolsRoot "authoring/$($lock.id)" }
$ScratchDir = [IO.Path]::GetFullPath($ScratchDir)
$OutputDir = [IO.Path]::GetFullPath($OutputDir)
if ($Jobs -lt 1 -or $Jobs -gt 32) { throw 'Jobs must be between 1 and 32.' }
$recipeFiles = @((Get-Item -LiteralPath $PSCommandPath)) + @(Get-ChildItem -LiteralPath $recipeRoot -File -Recurse | Where-Object { $_.Name -ne 'README.md' })
$recipeData = @($recipeFiles | Sort-Object FullName | ForEach-Object { [IO.Path]::GetRelativePath($PSScriptRoot,$_.FullName).Replace('\','/') + ':' + (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant() })
$recipeData += $ToolLock.gcc.sha256
$key = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes(($recipeData -join "`n")))).ToLowerInvariant()
$keyFile = Join-Path $OutputDir 'build-key.json'
$manifestFile = Join-Path $OutputDir 'binaries.json'
if (-not $Force -and (Test-Path -LiteralPath $keyFile) -and (Test-Path -LiteralPath $manifestFile)) {
    $cached = Get-Content -LiteralPath $keyFile -Raw | ConvertFrom-Json
    $valid = $cached.recipeKey -eq $key
    $cachedBinaries = @(Get-Content -LiteralPath $manifestFile -Raw | ConvertFrom-Json)
    if ($cachedBinaries.Count -ne 3) { $valid = $false }
    foreach ($entry in $cachedBinaries) {
        $path = Join-Path $OutputDir "bin/$($entry.name)"
        if (-not (Test-Path -LiteralPath $path) -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $entry.sha256) { $valid = $false }
    }
    if ($valid) {
        & (Join-Path $PSScriptRoot 'verify-toolchain-sources.ps1') -Directory (Join-Path $OutputDir 'sources') -BinaryManifest $manifestFile
        Write-Output "Reusing source-built DVD tools ($($lock.id))."
        return
    }
}
if ((Test-Path -LiteralPath $OutputDir) -and @(Get-ChildItem -LiteralPath $OutputDir -Force).Count) {
    throw 'The output directory is nonempty and cannot be reused. Use a new toolchain ID/output directory for changed recipes, or a different empty -OutputDir for a forced rebuild.'
}
$downloads = Join-Path $ScratchDir 'downloads'
$temp = Join-Path $ScratchDir 'temp'
New-Item -ItemType Directory -Force -Path $downloads,$temp | Out-Null
$oldTemp, $oldTmp, $oldPath = $env:TEMP, $env:TMP, $env:PATH
$env:TEMP = $temp
$env:TMP = $temp
function Fetch([string]$Name, [string]$URL, [string]$SHA256) {
    $target = Join-Path $downloads $Name
    if (-not (Test-Path -LiteralPath $target) -or (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $SHA256) {
        & curl.exe --fail --location --retry 2 --silent --show-error $URL --output $target
        Assert-Exit "Download $Name"
    }
    if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $SHA256) { throw "Checksum mismatch: $Name" }
    return $target
}
function Run-Logged([string]$Log, [string]$Exe, [string[]]$Arguments) {
    & $Exe @Arguments *> $Log
    if ($LASTEXITCODE -ne 0) { Get-Content -LiteralPath $Log -Tail 35 | Write-Host; throw "Build step failed; see $Log" }
}
try {
    $gccBin = Join-Path $ToolsRoot 'mingw64/bin'
    if (-not (Test-Path -LiteralPath (Join-Path $gccBin 'gcc.exe'))) {
        $compiler = Get-PinnedArchive $ToolLock.gcc
        & tar.exe -xf $compiler -C $ToolsRoot
        Assert-Exit 'Extract pinned C compiler'
    }
    $env:PATH = "$gccBin;$oldPath"
    $cmake = Join-Path $gccBin 'cmake.exe'
    $msysRoot = Join-Path $ScratchDir 'msys64'
    $bash = Join-Path $msysRoot 'usr/bin/bash.exe'
    if (-not (Test-Path -LiteralPath $bash)) {
        $archive = Fetch $lock.msys.archive $lock.msys.url $lock.msys.sha256
        & tar.exe -xf $archive -C $ScratchDir
        Assert-Exit 'Extract isolated MSYS build environment'
    }
    $work = Join-Path $ScratchDir ('build-' + [guid]::NewGuid().ToString('N'))
    $src = Join-Path $work 'src'
    $build = Join-Path $work 'build'
    $prefix = Join-Path $work 'prefix'
    New-Item -ItemType Directory -Force -Path $src,$build,$prefix | Out-Null
    $packages = @()
    foreach ($entry in $lock.developmentPackages) {
        $packages += Fetch $entry.archive $entry.url $entry.sha256
        $null = Fetch ($entry.archive+'.sig') ($entry.url+'.sig') $entry.signatureSha256
    }
    Run-Logged (Join-Path $build 'msys-keyring.log') $bash @('--noprofile','--norc','-c','export PATH=/usr/bin; pacman-key --init && pacman-key --populate msys2')
    Run-Logged (Join-Path $build 'msys-packages.log') $bash (@('--noprofile','--norc','-c','export PATH=/usr/bin; exec pacman -U --needed --noconfirm "$@"','install-build-tools') + $packages)
    foreach ($entry in $lock.sources) {
        $archive = Fetch $entry.name $entry.url $entry.sha256
        & tar.exe -xf $archive -C $src
        Assert-Exit "Extract $($entry.name)"
    }
    $dvdauthor = Join-Path $src 'dvdauthor'
    $cdrtools = Join-Path $src 'cdrtools-2.01'
    & (Join-Path $recipeRoot 'port-cdrtools.ps1') -Source $cdrtools
    Run-Logged (Join-Path $build 'dvdauthor-patch.log') $bash @('--noprofile','--norc','-c','export PATH=/usr/bin; exec patch --batch -p1 -d "$1" -i "$2"','apply-dvd-patch',$dvdauthor,(Join-Path $recipeRoot 'patches/dvdauthor-windows.patch'))
    $common = @('-G','MinGW Makefiles',"-DCMAKE_INSTALL_PREFIX=$prefix",'-DCMAKE_BUILD_TYPE=Release')
    function Build-Library([string]$Name, [string]$Source, [string[]]$Options) {
        $dir = Join-Path $build $Name
        $flags = '"-ffile-prefix-map=' + $work.Replace('\','/') + '=source-build"'
        Run-Logged (Join-Path $build "$Name-configure.log") $cmake (@('-S',$Source,'-B',$dir) + $common + @("-DCMAKE_C_FLAGS=$flags") + $Options)
        Run-Logged (Join-Path $build "$Name-build.log") $cmake @('--build',$dir,'--parallel',"$Jobs")
        Run-Logged (Join-Path $build "$Name-install.log") $cmake @('--install',$dir)
    }
    Build-Library 'zlib' (Join-Path $src 'zlib-1.3.2') @('-DZLIB_BUILD_SHARED=OFF','-DZLIB_BUILD_STATIC=ON','-DZLIB_BUILD_TESTING=OFF')
    Build-Library 'png' (Join-Path $src 'libpng-1.6.59') @("-DCMAKE_PREFIX_PATH=$prefix","-DZLIB_LIBRARY=$prefix/lib/libzs.a",'-DPNG_SHARED=OFF','-DPNG_STATIC=ON','-DPNG_TESTS=OFF','-DPNG_TOOLS=OFF')
    Build-Library 'xml' (Join-Path $src 'libxml2-2.15.4') @('-DBUILD_SHARED_LIBS=OFF','-DLIBXML2_WITH_CATALOG=OFF','-DLIBXML2_WITH_ICONV=OFF','-DLIBXML2_WITH_PROGRAMS=OFF','-DLIBXML2_WITH_TESTS=OFF','-DLIBXML2_WITH_PYTHON=OFF','-DLIBXML2_WITH_MODULES=OFF','-DLIBXML2_WITH_ZLIB=OFF')
    Build-Library 'dvd' $recipeRoot @("-DDVDAUTHOR_SOURCE=$dvdauthor","-DCMAKE_PREFIX_PATH=$prefix","-DZLIB_LIBRARY=$prefix/lib/libzs.a")
    Run-Logged (Join-Path $build 'mkisofs.log') $bash @('--noprofile','--norc',(Join-Path $recipeRoot 'build-cdrtools.sh'),$gccBin,$cdrtools,"$Jobs")
    Copy-Item -LiteralPath (Join-Path $cdrtools 'mkisofs/OBJ/x86_64-mingw32_nt-gcc/mkisofs.exe') -Destination (Join-Path $prefix 'bin/mkisofs.exe')
    $names = @('dvdauthor.exe','spumux.exe','mkisofs.exe')
    $manifest = @()
    foreach ($name in $names) {
        $file = Join-Path $prefix "bin/$name"
        & (Join-Path $gccBin 'strip.exe') --strip-all $file
        Assert-Exit "Strip $name"
        $bytes = [IO.File]::ReadAllBytes($file)
        $privatePaths = @($ScratchDir.Replace('\','/'),$ScratchDir.Replace('/','\'),$env:USERPROFILE,$env:USERPROFILE.Replace('\','/')) | Where-Object { $_ }
        foreach ($encoding in @([Text.Encoding]::UTF8,[Text.Encoding]::Unicode)) {
            $text = $encoding.GetString($bytes)
            foreach ($path in $privatePaths) { if ($text.Contains($path,[StringComparison]::OrdinalIgnoreCase)) { throw "Build path embedded in $name; refuse to publish this tool cache." } }
        }
        $imports = & (Join-Path $gccBin 'objdump.exe') -p $file
        Assert-Exit "Inspect $name imports"
        $dlls = @($imports | ForEach-Object { if ($_ -match 'DLL Name:\s*(\S+)') { $Matches[1] } })
        $unexpected = @($dlls | Where-Object { $_ -notmatch '^(?i:kernel32|user32|advapi32|ws2_32|bcrypt|msvcrt|ucrtbase)\.dll$|^(?i:api-ms-win-crt-).*\.dll$' })
        if ($unexpected.Count) { throw "Unexpected runtime dependencies in ${name}: $($unexpected -join ', ')" }
        $manifest += [ordered]@{name=$name;sha256=(Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant();imports=$dlls}
    }
    # Build a source companion with original archives, every local change and the recipe.
    $material = Join-Path $work 'material'
    New-Item -ItemType Directory -Force -Path $material,(Join-Path $material 'archives'),(Join-Path $material 'recipe/scripts'),(Join-Path $material 'notices') | Out-Null
    foreach ($entry in $lock.sources) { Copy-Item -LiteralPath (Join-Path $downloads $entry.name) -Destination (Join-Path $material 'archives') }
    foreach ($file in @('build-authoring.ps1','common.ps1','tools.lock.json','verify-toolchain-sources.ps1')) { Copy-Item -LiteralPath (Join-Path $PSScriptRoot $file) -Destination (Join-Path $material 'recipe/scripts') }
    Copy-Item -LiteralPath $recipeRoot -Destination (Join-Path $material 'recipe/scripts/toolchain') -Recurse
    Copy-Item -LiteralPath (Join-Path $ProjectRoot 'licenses') -Destination (Join-Path $material 'recipe/licenses') -Recurse
    Copy-Item -LiteralPath (Join-Path $recipeRoot 'README.md') -Destination (Join-Path $material 'BUILD.md')
    $noticeFiles = @{
        'GPL-2.0.txt'=(Join-Path $dvdauthor 'COPYING'); 'libxml2-Copyright.txt'=(Join-Path $src 'libxml2-2.15.4/Copyright');
        'libpng-LICENSE.txt'=(Join-Path $src 'libpng-1.6.59/LICENSE'); 'zlib-LICENSE.txt'=(Join-Path $src 'zlib-1.3.2/LICENSE')
    }
    foreach ($entry in $noticeFiles.GetEnumerator()) { Copy-Item -LiteralPath $entry.Value -Destination (Join-Path $material "notices/$($entry.Key)") }
    $archiveRecords = @($lock.sources | ForEach-Object { @{path='archives/'+$_.name;sha256=$_.sha256} })
    $patches = @('recipe/scripts/toolchain/patches/dvdauthor-windows.patch','recipe/scripts/toolchain/port-cdrtools.ps1','recipe/scripts/toolchain/CMakeLists.txt','recipe/scripts/toolchain/config.h.in','recipe/scripts/toolchain/image-only.c')
    $components = @($manifest | ForEach-Object { [ordered]@{binary=$_.name;binarySha256=$_.sha256;sourceArchives=$archiveRecords;licenseFiles=@('notices/GPL-2.0.txt','notices/libxml2-Copyright.txt','notices/libpng-LICENSE.txt','notices/zlib-LICENSE.txt');buildInstructions='BUILD.md';patchStatus='included';patchFiles=$patches} })
    $materialFiles = @(Get-ChildItem -LiteralPath $material -File -Recurse | ForEach-Object { [ordered]@{path=[IO.Path]::GetRelativePath($material,$_.FullName).Replace('\','/');sha256=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()} })
    [ordered]@{status='complete';toolchain=$lock.id;recipeKey=$key;compiler=$ToolLock.gcc;components=$components;materialFiles=$materialFiles} | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $material 'review.json') -Encoding utf8
    $temporaryManifest = Join-Path $work 'binaries.json'
    ConvertTo-Json -InputObject $manifest -Depth 4 | Set-Content -LiteralPath $temporaryManifest -Encoding utf8
    & (Join-Path $PSScriptRoot 'verify-toolchain-sources.ps1') -Directory $material -BinaryManifest $temporaryManifest
    New-Item -ItemType Directory -Force -Path $OutputDir,(Join-Path $OutputDir 'bin') | Out-Null
    foreach ($name in $names) { Copy-Item -LiteralPath (Join-Path $prefix "bin/$name") -Destination (Join-Path $OutputDir 'bin') -Force }
    Copy-Item -LiteralPath $material -Destination (Join-Path $OutputDir 'sources') -Recurse -Force
    Copy-Item -LiteralPath $temporaryManifest -Destination $manifestFile -Force
    [ordered]@{toolchain=$lock.id;recipeKey=$key;compiler=$ToolLock.gcc.sha256} | ConvertTo-Json | Set-Content -LiteralPath $keyFile -Encoding utf8
    Write-Output "Built self-contained DVD tools from source ($($lock.id))."
} finally { $env:TEMP=$oldTemp; $env:TMP=$oldTmp; $env:PATH=$oldPath }
