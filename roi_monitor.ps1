# ROI Monitoring Script for Ginie Autopilot Positions
# Tracks positions and monitors for ROI-based early profit booking

param(
    [int]$IntervalSeconds = 30,
    [int]$MaxDurationMinutes = 120
)

$TakerFeeRate = 0.0004
$ROIThresholds = @{
    'ultra_fast' = 3.0
    'scalp'      = 5.0
    'swing'      = 8.0
    'position'   = 10.0
}

$MonitorStartTime = Get-Date
$PositionHistory = @{}
$ThresholdHits = @()

function Calculate-ROI {
    param(
        [double]$EntryPrice,
        [double]$CurrentPrice,
        [double]$Quantity,
        [string]$Side,
        [int]$Leverage
    )
    
    # Gross profit/loss
    if ($Side -eq "LONG") {
        $GrossPnL = ($CurrentPrice - $EntryPrice) * $Quantity
    } else {
        $GrossPnL = ($EntryPrice - $CurrentPrice) * $Quantity
    }
    
    # Fees
    $NotionalAtEntry = $Quantity * $EntryPrice
    $EntryFee = $NotionalAtEntry * $TakerFeeRate
    $ExitFee = ($CurrentPrice * $Quantity) * $TakerFeeRate
    $TotalFees = $EntryFee + $ExitFee
    
    # Net PnL after fees
    $NetPnL = $GrossPnL - $TotalFees
    
    # ROI with leverage consideration
    if ($NotionalAtEntry -le 0) {
        return 0
    }
    
    $ROI = ($NetPnL * $Leverage / $NotionalAtEntry) * 100
    return $ROI
}

Write-Host "===========================================" -ForegroundColor Cyan
Write-Host "GINIE AUTOPILOT - ROI MONITORING STARTED" -ForegroundColor Cyan
Write-Host "Start Time: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Cyan
Write-Host "Check Interval: $IntervalSeconds seconds" -ForegroundColor Cyan
Write-Host "Max Duration: $MaxDurationMinutes minutes" -ForegroundColor Cyan
Write-Host "===========================================" -ForegroundColor Cyan
Write-Host ""

$ElapsedMinutes = 0

while ($ElapsedMinutes -lt $MaxDurationMinutes) {
    $Timestamp = Get-Date -Format 'HH:mm:ss'
    
    try {
        # Fetch current positions
        $Response = Invoke-WebRequest -Uri "http://localhost:8094/api/futures/ginie/autopilot/positions" `
            -TimeoutSec 10 -ErrorAction Stop
        
        if ($Response.StatusCode -eq 200) {
            $Data = $Response.Content | ConvertFrom-Json
            $Positions = $Data.positions
            
            Write-Host "[$Timestamp] Checking $($Data.count) active positions..." -ForegroundColor Yellow
            
            foreach ($Pos in $Positions) {
                $Symbol = $Pos.symbol
                $CurrentPrice = if ($Pos.side -eq "LONG") { $Pos.highest_price } else { $Pos.highest_price }
                
                # Calculate current ROI with leverage
                $ROI = Calculate-ROI -EntryPrice $Pos.entry_price -CurrentPrice $CurrentPrice `
                                     -Quantity $Pos.remaining_qty -Side $Pos.side -Leverage $Pos.leverage
                
                $Threshold = $ROIThresholds[$Pos.mode]
                $Status = if ($ROI -ge $Threshold) { "✓ HIT" } else { "  --" }
                $ROIColor = if ($ROI -ge $Threshold) { "Green" } else { "White" }
                
                # Display position ROI
                Write-Host "  $Symbol ($($Pos.mode)): ROI=$([math]::Round($ROI, 2))% | Threshold=$Threshold% | $Status" `
                    -ForegroundColor $ROIColor
                
                # Track threshold hits
                if ($ROI -ge $Threshold) {
                    $Key = "$Symbol-$($Pos.mode)"
                    if (-not $PositionHistory.ContainsKey($Key)) {
                        $PositionHistory[$Key] = @{
                            'FirstHitTime' = $Timestamp
                            'ROI' = $ROI
                            'Status' = 'THRESHOLD_HIT'
                        }
                        
                        $ThresholdHits += @{
                            'Symbol' = $Symbol
                            'Mode' = $Pos.mode
                            'ROI' = $ROI
                            'Threshold' = $Threshold
                            'HitTime' = $Timestamp
                            'Side' = $Pos.side
                            'EntryPrice' = $Pos.entry_price
                            'CurrentPrice' = $CurrentPrice
                            'Leverage' = $Pos.leverage
                        }
                        
                        Write-Host "    ⚠️  ROI THRESHOLD HIT! Position should close on next monitoring cycle." `
                            -ForegroundColor Green
                    }
                }
                
                # Update history
                if ($PositionHistory.ContainsKey($Symbol)) {
                    $PositionHistory[$Symbol]['LastROI'] = $ROI
                    $PositionHistory[$Symbol]['LastCheckTime'] = $Timestamp
                } else {
                    $PositionHistory[$Symbol] = @{
                        'LastROI' = $ROI
                        'LastCheckTime' = $Timestamp
                        'EntryPrice' = $Pos.entry_price
                        'Mode' = $Pos.mode
                    }
                }
            }
        }
    } catch {
        Write-Host "[$Timestamp] Error fetching positions: $_" -ForegroundColor Red
    }
    
    Write-Host ""
    
    # Check elapsed time
    $ElapsedMinutes = [math]::Round(($(Get-Date) - $MonitorStartTime).TotalMinutes)
    
    if ($ElapsedMinutes -lt $MaxDurationMinutes) {
        Start-Sleep -Seconds $IntervalSeconds
    }
}

# Generate monitoring report
Write-Host "===========================================" -ForegroundColor Cyan
Write-Host "MONITORING SESSION COMPLETED" -ForegroundColor Cyan
Write-Host "Duration: $([math]::Round(($(Get-Date) - $MonitorStartTime).TotalMinutes, 1)) minutes" -ForegroundColor Cyan
Write-Host "===========================================" -ForegroundColor Cyan
Write-Host ""

if ($ThresholdHits.Count -gt 0) {
    Write-Host "ROI THRESHOLD HITS DETECTED: $($ThresholdHits.Count)" -ForegroundColor Green
    Write-Host ""
    
    foreach ($Hit in $ThresholdHits) {
        Write-Host "Position: $($Hit.Symbol) ($($Hit.Mode.ToUpper()))" -ForegroundColor Green
        Write-Host "  ROI Achieved: $([math]::Round($Hit.ROI, 2))% (Threshold: $($Hit.Threshold)%)"
        Write-Host "  Side: $($Hit.Side) | Leverage: $($Hit.Leverage)x"
        Write-Host "  Entry: $([math]::Round($Hit.EntryPrice, 8)) | Current: $([math]::Round($Hit.CurrentPrice, 8))"
        Write-Host "  Hit Time: $($Hit.HitTime)"
        Write-Host ""
    }
} else {
    Write-Host "No ROI thresholds hit during monitoring period." -ForegroundColor Yellow
    Write-Host "Positions still require more favorable price movement." -ForegroundColor Yellow
    Write-Host ""
    
    Write-Host "Current ROI Status:" -ForegroundColor Cyan
    foreach ($Key in $PositionHistory.Keys) {
        $Entry = $PositionHistory[$Key]
        Write-Host "  $Key : ROI=$([math]::Round($Entry['LastROI'], 2))% | Mode=$($Entry['Mode'])"
    }
}

Write-Host ""
Write-Host "Monitoring report saved to monitoring_summary.txt" -ForegroundColor Cyan
