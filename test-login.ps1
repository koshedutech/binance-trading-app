$body = @{
    email = "test@example.com"
    password = "TestPass123"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "http://localhost:8088/api/auth/login" -Method POST -ContentType "application/json" -Body $body
    Write-Host "Success:"
    $response | ConvertTo-Json -Depth 10
} catch {
    Write-Host "Error:" $_.Exception.Message
    Write-Host "Status:" $_.Exception.Response.StatusCode.Value__
    Write-Host "Details:" $_.ErrorDetails.Message
}
