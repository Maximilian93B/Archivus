# Debug Documents Response
Write-Host "DEBUGGING DOCUMENTS ENDPOINT RESPONSE" -ForegroundColor Yellow
Write-Host "======================================"

# Login first
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$loginData = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $loginResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $headers -Body $loginData
    Write-Host "✅ Login successful!" -ForegroundColor Green
    $token = $loginResponse.token
} catch {
    Write-Host "❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test documents endpoint with detailed response
Write-Host "`nTesting Documents Endpoint..." -ForegroundColor Cyan
$authHeaders = @{ 
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

try {
    $documentsResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/documents" -Method GET -Headers $authHeaders
    Write-Host "✅ Documents endpoint accessible!" -ForegroundColor Green
    Write-Host "Response structure:" -ForegroundColor Yellow
    $documentsResponse | ConvertTo-Json -Depth 3 | Write-Host
} catch {
    Write-Host "❌ Documents endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "Status Code: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    
    # Try to get more details
    try {
        $errorResponse = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($errorResponse)
        $errorBody = $reader.ReadToEnd()
        Write-Host "Error Body: $errorBody" -ForegroundColor Red
    } catch {
        Write-Host "Could not read error response" -ForegroundColor Red
    }
}

Write-Host "`nDebug Complete" -ForegroundColor Yellow 