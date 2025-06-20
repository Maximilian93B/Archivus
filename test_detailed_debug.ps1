# Detailed Debug Test
Write-Host "DETAILED AUTHENTICATION DEBUG" -ForegroundColor Yellow
Write-Host "==============================" 

# Test 1: Login
Write-Host "`n1. Testing Login..." -ForegroundColor Cyan
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$loginData = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $loginResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $headers -Body $loginData
    Write-Host "✅ Login successful!" -ForegroundColor Green
    $token = $loginResponse.token
    Write-Host "   Token length: $($token.Length)" -ForegroundColor Gray
    Write-Host "   Token start: $($token.Substring(0,50))..." -ForegroundColor Gray
} catch {
    Write-Host "❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test 2: Documents endpoint with verbose error handling
Write-Host "`n2. Testing Documents Endpoint with detailed error handling..." -ForegroundColor Cyan
$authHeaders = @{ 
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

Write-Host "   Using headers:" -ForegroundColor Gray
$authHeaders.GetEnumerator() | ForEach-Object { 
    if ($_.Key -eq "Authorization") {
        Write-Host "     $($_.Key): Bearer $($token.Substring(0,20))..." -ForegroundColor Gray
    } else {
        Write-Host "     $($_.Key): $($_.Value)" -ForegroundColor Gray
    }
}

try {
    Write-Host "   Making request to: http://localhost:8080/api/v1/documents" -ForegroundColor Gray
    $documentsResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/documents" -Method GET -Headers $authHeaders
    Write-Host "✅ Documents endpoint SUCCESS!" -ForegroundColor Green
    Write-Host "   Response type: $($documentsResponse.GetType().Name)" -ForegroundColor Gray
    Write-Host "   Response content:" -ForegroundColor Gray
    $documentsResponse | ConvertTo-Json -Depth 2 | Write-Host -ForegroundColor Gray
} catch {
    Write-Host "❌ Documents endpoint FAILED!" -ForegroundColor Red
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "   Status Code: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    Write-Host "   Status Description: $($_.Exception.Response.StatusDescription)" -ForegroundColor Red
    
    # Try to get response body
    try {
        $responseStream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($responseStream)
        $responseBody = $reader.ReadToEnd()
        Write-Host "   Response Body: $responseBody" -ForegroundColor Red
    } catch {
        Write-Host "   Could not read response body" -ForegroundColor Red
    }
    
    # Show request details
    Write-Host "   Request URL: http://localhost:8080/api/v1/documents" -ForegroundColor Red
    Write-Host "   Request Method: GET" -ForegroundColor Red
    Write-Host "   Request Headers:" -ForegroundColor Red
    $authHeaders.GetEnumerator() | ForEach-Object { 
        Write-Host "     $($_.Key): $($_.Value)" -ForegroundColor Red
    }
}

Write-Host "`nDebug test complete" -ForegroundColor Yellow 