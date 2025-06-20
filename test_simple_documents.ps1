Write-Host "SIMPLE DOCUMENTS ENDPOINT TEST" -ForegroundColor Yellow
Write-Host "===============================" -ForegroundColor Yellow

$baseUrl = "http://localhost:8080"

# Step 1: Login
Write-Host "`n1. Login..." -ForegroundColor Cyan
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$body = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method POST -Headers $headers -Body $body
    Write-Host "✅ Login successful!" -ForegroundColor Green
    $token = $response.token
} catch {
    Write-Host "❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Step 2: Test Documents List Endpoint
Write-Host "`n2. Testing Documents List..." -ForegroundColor Cyan
$authHeaders = @{ 
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

try {
    $documents = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents" -Method GET -Headers $authHeaders
    Write-Host "✅ Documents list successful!" -ForegroundColor Green
    Write-Host "   Found $($documents.documents.Count) documents" -ForegroundColor White
    
    if ($documents.documents.Count -gt 0) {
        Write-Host "   First document:" -ForegroundColor Gray
        $documents.documents[0] | ConvertTo-Json -Depth 2 | Write-Host -ForegroundColor White
    }
    
} catch {
    Write-Host "❌ Documents list failed!" -ForegroundColor Red
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "   Status: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    
    # Try to get response body
    try {
        $responseStream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($responseStream)
        $responseBody = $reader.ReadToEnd()
        Write-Host "   Response Body: $responseBody" -ForegroundColor Red
    } catch {
        Write-Host "   Could not read response body" -ForegroundColor Red
    }
}

Write-Host "`nSimple documents test complete." -ForegroundColor Yellow 