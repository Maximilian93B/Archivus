# Debug Token Validation
# This script helps debug why token validation is failing

Write-Host "DEBUG: Token Validation Analysis" -ForegroundColor Cyan
Write-Host "=" * 40 -ForegroundColor Cyan

$baseUrl = "http://localhost:8080"

# Use the most recent tenant we created
$tenantSubdomain = "test20250618144954"
$adminEmail = "admin20250618144954@test.com"

Write-Host "Using tenant: $tenantSubdomain" -ForegroundColor Yellow
Write-Host "Using email: $adminEmail" -ForegroundColor Yellow

# Step 1: Login to get token
Write-Host "`nStep 1: Getting token..." -ForegroundColor Magenta

$loginPayload = @{
    email = $adminEmail
    password = "SecurePass123!"
} | ConvertTo-Json

$loginHeaders = @{
    "X-Tenant-Subdomain" = $tenantSubdomain
}

try {
    $loginResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/auth/login" -Method POST -Body $loginPayload -ContentType "application/json" -Headers $loginHeaders
    $loginData = $loginResponse.Content | ConvertFrom-Json
    $token = $loginData.token
    
    Write-Host "SUCCESS: Token obtained" -ForegroundColor Green
    Write-Host "Token (first 50 chars): $($token.Substring(0, 50))..." -ForegroundColor Cyan
    Write-Host "User ID: $($loginData.user.id)" -ForegroundColor Cyan
    Write-Host "Email: $($loginData.user.email)" -ForegroundColor Cyan
    Write-Host "Role: $($loginData.user.role)" -ForegroundColor Cyan
    Write-Host "Is Active: $($loginData.user.is_active)" -ForegroundColor Cyan
    Write-Host "Tenant ID: $($loginData.user.tenant_id)" -ForegroundColor Cyan
} catch {
    Write-Host "FAILED: Cannot get token - $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Step 2: Test the token validation endpoint directly
Write-Host "`nStep 2: Testing token validation endpoint..." -ForegroundColor Magenta

$authHeaders = @{
    "Authorization" = "Bearer $token"
    "X-Tenant-Subdomain" = $tenantSubdomain
}

try {
    $validateResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/auth/validate" -Method GET -Headers $authHeaders
    Write-Host "SUCCESS: Token validation passed (Status: $($validateResponse.StatusCode))" -ForegroundColor Green
    Write-Host "Response: $($validateResponse.Content)" -ForegroundColor Cyan
} catch {
    Write-Host "INFO: Token validation endpoint test" -ForegroundColor Yellow
    Write-Host "Status: $($_.Exception.Response.StatusCode)" -ForegroundColor Yellow
    Write-Host "Message: $($_.Exception.Message)" -ForegroundColor Yellow
    
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $responseBody = $reader.ReadToEnd()
        Write-Host "Response Body: $responseBody" -ForegroundColor Yellow
    }
}

# Step 3: Test different authenticated endpoints
Write-Host "`nStep 3: Testing various authenticated endpoints..." -ForegroundColor Magenta

$endpoints = @(
    "/api/v1/users/profile",
    "/api/v1/tenant/settings",
    "/api/v1/documents/",
    "/api/v1/folders"
)

foreach ($endpoint in $endpoints) {
    Write-Host "`nTesting: $endpoint" -ForegroundColor Yellow
    
    try {
        $response = Invoke-WebRequest -Uri "$baseUrl$endpoint" -Method GET -Headers $authHeaders
        Write-Host "SUCCESS: $endpoint (Status: $($response.StatusCode))" -ForegroundColor Green
    } catch {
        Write-Host "FAILED: $endpoint (Status: $($_.Exception.Response.StatusCode))" -ForegroundColor Red
        
        if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $responseBody = $reader.ReadToEnd()
            Write-Host "Error Body: $responseBody" -ForegroundColor Red
        }
    }
}

# Step 4: Test without tenant header
Write-Host "`nStep 4: Testing without tenant header..." -ForegroundColor Magenta

$authHeadersNoTenant = @{
    "Authorization" = "Bearer $token"
}

try {
    $response = Invoke-WebRequest -Uri "$baseUrl/api/v1/users/profile" -Method GET -Headers $authHeadersNoTenant
    Write-Host "SUCCESS: Without tenant header (Status: $($response.StatusCode))" -ForegroundColor Green
} catch {
    Write-Host "FAILED: Without tenant header (Status: $($_.Exception.Response.StatusCode))" -ForegroundColor Red
    
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $responseBody = $reader.ReadToEnd()
        Write-Host "Error Body: $responseBody" -ForegroundColor Red
    }
}

Write-Host "`nDebug analysis completed!" -ForegroundColor Cyan