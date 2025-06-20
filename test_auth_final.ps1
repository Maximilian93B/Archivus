# Final Authentication Test
Write-Host "FINAL AUTHENTICATION TEST" -ForegroundColor Cyan
Write-Host "========================="

try {
    # Step 1: Login
    Write-Host "1. Testing Login..." -ForegroundColor Yellow
    $headers = @{ 
        "Content-Type" = "application/json"
        "X-Tenant-Subdomain" = "testdebug123" 
    }
    $loginData = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

    $loginResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $headers -Body $loginData
    Write-Host "✅ Login successful!" -ForegroundColor Green
    $token = $loginResponse.token
    Write-Host "   Token: $($token.Substring(0,50))..." -ForegroundColor Gray

    # Step 2: Test Documents Endpoint
    Write-Host "`n2. Testing Documents Endpoint..." -ForegroundColor Yellow
    $authHeaders = @{ 
        "Authorization" = "Bearer $token"
        "Content-Type" = "application/json"
    }

    $documentsResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/documents" -Method GET -Headers $authHeaders
    Write-Host "✅ Documents endpoint SUCCESS!" -ForegroundColor Green
    
    # Show response details
    if ($documentsResponse) {
        Write-Host "   Response type: $($documentsResponse.GetType().Name)" -ForegroundColor Gray
        if ($documentsResponse.data) {
            Write-Host "   Found $($documentsResponse.data.Count) documents" -ForegroundColor Gray
        } elseif ($documentsResponse -is [array]) {
            Write-Host "   Found $($documentsResponse.Count) documents" -ForegroundColor Gray
        } else {
            Write-Host "   Response: $($documentsResponse | ConvertTo-Json -Compress)" -ForegroundColor Gray
        }
    }

    Write-Host "`n🎉 AUTHENTICATION FULLY WORKING!" -ForegroundColor Green
    Write-Host "Ready to test Phase 2 pipeline!" -ForegroundColor Green

} catch {
    Write-Host "❌ Test failed: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        Write-Host "   Status: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    }
    exit 1
} 