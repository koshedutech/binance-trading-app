$d = Invoke-RestMethod 'http://localhost:8090/api/futures/ginie/status'
Write-Host "=== GINIE DECISIONS ===" -ForegroundColor Cyan
Write-Host "Time: $(Get-Date -Format 'HH:mm:ss') | Total: $($d.recent_decisions.Count) decisions"
Write-Host ""
Write-Host "Symbol       | Status          | Mode     | Conf   | Action" -ForegroundColor Yellow
Write-Host "-------------|-----------------|----------|--------|--------"
$d.recent_decisions | Select-Object -First 10 | ForEach-Object {
    $conf = [math]::Round($_.confidence_score, 1)
    Write-Host "$($_.symbol.PadRight(12)) | $($_.scan_status.PadRight(15)) | $($_.selected_mode.PadRight(8)) | $($conf.ToString().PadLeft(5))% | $($_.recommendation)"
}
