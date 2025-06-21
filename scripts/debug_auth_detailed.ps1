Write-Host "DETAILED AUTHENTICATION DEBUG" -ForegroundColor Yellow
Write-Host "=============================" -ForegroundColor Yellow

# Test server health first
Write-Host "`n1. Testing Server Health..." -ForegroundColor Cyan
try {
    $healthResponse = Invoke-RestMethod -Uri "http://localhost:8080/health" -Method GET
    Write-Host "✅ Server is healthy" -ForegroundColor Green
    Write-Host "   Response: $($healthResponse | ConvertTo-Json -Compress)" -ForegroundColor Gray
} catch {
    Write-Host "❌ Server health check failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test login
Write-Host "`n2. Testing Login..." -ForegroundColor Cyan
$loginBody = @{
    email = "admin@testdebug123.com"
    password = "SecurePass123!"
} | ConvertTo-Json

$headers = @{
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123"
}

try {
    $loginResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Body $loginBody -Headers $headers
    Write-Host "✅ Login successful!" -ForegroundColor Green
    $accessToken = $loginResponse.token
    Write-Host "   Token: $($accessToken.Substring(0, 50))..." -ForegroundColor Gray
    Write-Host "   User ID: $($loginResponse.user.id)" -ForegroundColor Gray
    Write-Host "   Tenant ID: $($loginResponse.user.tenant_id)" -ForegroundColor Gray
    Write-Host "   Role: $($loginResponse.user.role)" -ForegroundColor Gray
} catch {
    Write-Host "❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $errorStream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($errorStream)
        $errorBody = $reader.ReadToEnd()
        Write-Host "   Error details: $errorBody" -ForegroundColor Red
    }
    exit 1
}

# Test token validation endpoint
Write-Host "`n3. Testing Token Validation Endpoint..." -ForegroundColor Cyan
$authHeaders = @{
    "Authorization" = "Bearer $accessToken"
    "Content-Type" = "application/json"
}

try {
    $validateResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/validate" -Method GET -Headers $authHeaders
    Write-Host "✅ Token validation successful!" -ForegroundColor Green
    Write-Host "   User ID: $($validateResponse.user_id)" -ForegroundColor Gray
    Write-Host "   Tenant ID: $($validateResponse.tenant_id)" -ForegroundColor Gray
    Write-Host "   Email: $($validateResponse.email)" -ForegroundColor Gray
    Write-Host "   Role: $($validateResponse.role)" -ForegroundColor Gray
    Write-Host "   Is Active: $($validateResponse.is_active)" -ForegroundColor Gray
} catch {
    Write-Host "❌ Token validation failed: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $errorStream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($errorStream)
        $errorBody = $reader.ReadToEnd()
        Write-Host "   Error details: $errorBody" -ForegroundColor Red
    }
}

# Test documents endpoint with detailed error handling
Write-Host "`n4. Testing Documents Endpoint..." -ForegroundColor Cyan
try {
    $documentsResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/documents" -Method GET -Headers $authHeaders
    Write-Host "✅ Documents endpoint successful!" -ForegroundColor Green
    Write-Host "   Documents count: $($documentsResponse.data.Count)" -ForegroundColor Gray
} catch {
    Write-Host "❌ Documents endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "   Status Code: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    
    if ($_.Exception.Response) {
        try {
            $errorStream = $_.Exception.Response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($errorStream)
            $errorBody = $reader.ReadToEnd()
            Write-Host "   Error details: $errorBody" -ForegroundColor Red
            
            # Try to parse as JSON for better formatting
            try {
                $errorJson = $errorBody | ConvertFrom-Json
                Write-Host "   Parsed error:" -ForegroundColor Red
                Write-Host "     Error: $($errorJson.error)" -ForegroundColor Red
                Write-Host "     Message: $($errorJson.message)" -ForegroundColor Red
                if ($errorJson.details) {
                    Write-Host "     Details: $($errorJson.details)" -ForegroundColor Red
                }
            } catch {
                # If not JSON, just show raw
                Write-Host "   Raw error: $errorBody" -ForegroundColor Red
            }
        } catch {
            Write-Host "   Could not read error response body" -ForegroundColor Red
        }
    }
}

Write-Host "`nDebug completed." -ForegroundColor Yellow 