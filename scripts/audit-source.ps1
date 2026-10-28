# Audit files Git would publish, including untracked files before the first commit.
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
Push-Location $root
try {
    $files = @(& git ls-files --cached --others --exclude-standard)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot enumerate Git source files.' }
    $issues = @()
    foreach ($file in $files) {
        if ($file -match '(^|/)(AGENTS\.md|\.env(?:\..*)?|LOCAL-BACKGROUNDS\.md)$|^(output|bin|dist|\.tools)/|\.(exe|iso|db|syso|pem|key)$') {
            $issues += "Private or generated file in source list: $file"
            continue
        }
        if ($file -match '\.(png|ico)$') { continue }
        $content = Get-Content -LiteralPath $file -Raw
        $patterns = @(
            '(?i)[A-Z]:[\\/]Users[\\/][^\\/\s]+',
            '(?i)[A-Z]:[\\/]tmp[\\/]',
            '(?i)[A-Z]:[\\/][^\s`]*testt?data',
            '/(?:home|Users)/[A-Za-z0-9_.-]+/',
            '-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',
            '(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,}|AKIA[A-Z0-9]{16})',
            'https?://[^/\s:]+:[^/\s@]+@'
        )
        foreach ($pattern in $patterns) { if ($content -match $pattern) { $issues += "Sensitive/local pattern in $file"; break } }
    }
    if ($issues.Count) { throw ($issues -join "`n") }
    Write-Output "Source audit passed ($($files.Count) publishable files)."
} finally { Pop-Location }
