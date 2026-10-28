. (Join-Path $PSScriptRoot 'common.ps1')
foreach ($item in @(@{entry=$ToolLock.go; test='go/bin/go.exe'}, @{entry=$ToolLock.gcc; test='mingw64/bin/gcc.exe'})) {
    if (-not (Test-Path -LiteralPath (Join-Path $ToolsRoot $item.test))) {
        $archive = Get-PinnedArchive $item.entry
        & tar.exe -xf $archive -C $ToolsRoot
        Assert-Exit 'Extract development tools'
    }
}
Use-DevTools
Push-Location $ProjectRoot
try {
    & go version
    & go mod download
    Assert-Exit 'Download Go modules'
    & (Join-Path $PSScriptRoot 'prepare-tools.ps1')
} finally { Pop-Location }
