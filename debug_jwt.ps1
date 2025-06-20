Write-Host "JWT TOKEN DECODER" -ForegroundColor Cyan
Write-Host "=================" -ForegroundColor Cyan

# Test login to get token
$headers = @{ "Content-Type" = "application/json"; "X-Tenant-Subdomain" = "testdebug123" }
$body = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $headers -Body $body
    $token = $response.token
    Write-Host "✅ Got token: $($token.Substring(0,50))..." -ForegroundColor Green
    
    # Split JWT into parts
    $parts = $token -split '\.'
    if ($parts.Length -ne 3) {
        Write-Host "❌ Invalid JWT format - expected 3 parts, got $($parts.Length)" -ForegroundColor Red
        exit
    }
    
    Write-Host "`nJWT PARTS:" -ForegroundColor Yellow
    Write-Host "Header:    $($parts[0])" -ForegroundColor White
    Write-Host "Payload:   $($parts[1])" -ForegroundColor White  
    Write-Host "Signature: $($parts[2].Substring(0,20))..." -ForegroundColor White
    
    # Decode header (add padding if needed)
    $headerPadded = $parts[0]
    while ($headerPadded.Length % 4 -ne 0) { $headerPadded += "=" }
    try {
        $headerBytes = [Convert]::FromBase64String($headerPadded)
        $headerJson = [System.Text.Encoding]::UTF8.GetString($headerBytes)
        Write-Host "`nHEADER DECODED:" -ForegroundColor Yellow
        Write-Host $headerJson -ForegroundColor White
    } catch {
        Write-Host "❌ Failed to decode header: $($_.Exception.Message)" -ForegroundColor Red
    }
    
    # Decode payload (add padding if needed)
    $payloadPadded = $parts[1]
    while ($payloadPadded.Length % 4 -ne 0) { $payloadPadded += "=" }
    try {
        $payloadBytes = [Convert]::FromBase64String($payloadPadded)
        $payloadJson = [System.Text.Encoding]::UTF8.GetString($payloadBytes)
        Write-Host "`nPAYLOAD DECODED:" -ForegroundColor Yellow
        Write-Host $payloadJson -ForegroundColor White
        
        # Parse payload to check specific fields
        $payload = $payloadJson | ConvertFrom-Json
        Write-Host "`nKEY FIELDS:" -ForegroundColor Yellow
        Write-Host "Algorithm (from header): Check above" -ForegroundColor White
        Write-Host "Subject (sub): $($payload.sub)" -ForegroundColor White
        Write-Host "Email: $($payload.email)" -ForegroundColor White
        Write-Host "Issuer (iss): $($payload.iss)" -ForegroundColor White
        Write-Host "Audience (aud): $($payload.aud)" -ForegroundColor White
        Write-Host "Expires (exp): $($payload.exp)" -ForegroundColor White
        if ($payload.exp) {
            $expDate = [DateTimeOffset]::FromUnixTimeSeconds($payload.exp).DateTime
            Write-Host "Expires Date: $expDate" -ForegroundColor White
        }
    } catch {
        Write-Host "❌ Failed to decode payload: $($_.Exception.Message)" -ForegroundColor Red
    }
    
} catch {
    Write-Host "❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`nDone." -ForegroundColor Cyan 