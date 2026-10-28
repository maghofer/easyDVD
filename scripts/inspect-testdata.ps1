#requires -Version 7.0
param(
    [Parameter(Mandatory)][string]$Source,
    [Parameter(Mandatory)][string]$Scratch,
    [string]$FFprobe = 'ffprobe.exe'
)
$ErrorActionPreference = 'Stop'
$sourceRoot = (Get-Item -LiteralPath $Source).FullName
$reportRoot = [IO.Path]::GetFullPath($Scratch)
New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null
$probeExe = (Get-Command $FFprobe -CommandType Application | Select-Object -First 1).Source
$extensions = '.mp4 .mov .mkv .avi .mpg .mpeg .m2v .m4v .mts .m2ts .ts .vob .wmv .webm .dv .flv .3gp .mxf .ogv .mpv .divx .asf .m2t'.Split(' ')
$files = @(Get-ChildItem -LiteralPath $sourceRoot -File -Recurse | Where-Object { $_.Extension.ToLowerInvariant() -in $extensions } | Sort-Object FullName)
$records = [Collections.Generic.List[object]]::new()
foreach ($file in $files) {
    $relative = [IO.Path]::GetRelativePath($sourceRoot, $file.FullName)
    $record = [ordered]@{relative=$relative; bytes=$file.Length; mtime_utc=$file.LastWriteTimeUtc.ToString('o'); streams=@(); duration=0; error=$null}
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $probeExe
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    foreach ($argument in @('-v','error','-show_format','-show_streams','-of','json',$file.FullName)) { $start.ArgumentList.Add($argument) }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $start
    try {
        [void]$process.Start()
        $stdout = $process.StandardOutput.ReadToEndAsync()
        $stderr = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit(45000)) {
            $process.Kill($true)
            $process.WaitForExit()
            throw 'FFprobe timed out after 45 seconds'
        }
        $diagnostic = $stderr.GetAwaiter().GetResult()
        if ($process.ExitCode -ne 0) { throw $diagnostic }
        $probe = $stdout.GetAwaiter().GetResult() | ConvertFrom-Json
        $record.duration = [double]::Parse($probe.format.duration, [Globalization.CultureInfo]::InvariantCulture)
        $record.streams = @($probe.streams | Select-Object index,codec_type,codec_name,width,height,pix_fmt,r_frame_rate,avg_frame_rate,sample_aspect_ratio,display_aspect_ratio,field_order,color_range,color_space,color_transfer,color_primaries,sample_rate,channels)
        if ($diagnostic) { $record.error = $diagnostic.Trim() }
    } catch { $record.error = $_.Exception.Message }
    finally { $process.Dispose() }
    $records.Add([pscustomobject]$record)
    Write-Host ('[{0}/{1}] {2}: {3:N1}s {4}' -f $records.Count,$files.Count,$relative,$record.duration,$record.error)
    # Checkpoint after each probe so interrupted network inventories remain useful.
    $partial = Join-Path $reportRoot 'inventory.partial.json'
    ConvertTo-Json -InputObject $records.ToArray() -Depth 8 | Set-Content -LiteralPath $partial -Encoding utf8
    Move-Item -LiteralPath $partial -Destination (Join-Path $reportRoot 'inventory.json') -Force
}
$summary = @($records | Group-Object { ($_.relative -split '[\\/]')[0] } | ForEach-Object {
    [pscustomobject]@{folder=$_.Name;files=$_.Count;bytes=($_.Group | Measure-Object bytes -Sum).Sum;seconds=($_.Group | Measure-Object duration -Sum).Sum;errors=@($_.Group | Where-Object error).Count}
})
ConvertTo-Json -InputObject $summary -Depth 4 | Set-Content -LiteralPath (Join-Path $reportRoot 'summary.json') -Encoding utf8
$summary | Format-Table -AutoSize
