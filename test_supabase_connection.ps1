Write-Host "SUPABASE CONNECTION TEST" -ForegroundColor Cyan
Write-Host "=========================" -ForegroundColor Cyan

# Read current configuration
$env = Get-Content .env | ForEach-Object {
    if ($_ -match '^([^=]+)=(.*)$') {
        @{ $matches[1] = $matches[2] }
    }
} | ForEach-Object { $_ } | Group-Object -Property Keys | ForEach-Object { @{ $_.Name = $_.Group.Values[0] } }

Write-Host "`nCurrent Configuration:" -ForegroundColor Yellow
Write-Host "SUPABASE_URL: $($env.SUPABASE_URL)" -ForegroundColor White
Write-Host "SUPABASE_API_KEY: $($env.SUPABASE_API_KEY.Substring(0,50))..." -ForegroundColor White
Write-Host "JWT_SECRET: $($env.JWT_SECRET.Substring(0,20))..." -ForegroundColor White

Write-Host "`nTesting server health..." -ForegroundColor Yellow
try {
    $health = Invoke-RestMethod -Uri "http://localhost:8080/health"
    Write-Host "✅ Server is running: $($health.status)" -ForegroundColor Green
} catch {
    Write-Host "❌ Server health check failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host "`nTesting login with testdebug123 tenant..." -ForegroundColor Yellow
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$body = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $headers -Body $body
    Write-Host "✅ Login successful!" -ForegroundColor Green
    Write-Host "User: $($response.user.email)" -ForegroundColor White
    Write-Host "Tenant: $($response.user.tenant_id)" -ForegroundColor White
    Write-Host "Token: $($response.token.Substring(0,50))..." -ForegroundColor White
    
    # Test token validation by calling a protected endpoint
    Write-Host "`nTesting token validation..." -ForegroundColor Yellow
    $authHeaders = @{ 
        "Authorization" = "Bearer $($response.token)"
        "Content-Type" = "application/json"
    }
    
    try {
        $docs = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/documents" -Method GET -Headers $authHeaders
        Write-Host "✅ Token validation successful!" -ForegroundColor Green
        Write-Host "Documents endpoint accessible" -ForegroundColor White
    } catch {
        Write-Host "❌ Token validation failed: $($_.Exception.Message)" -ForegroundColor Red
        Write-Host "This indicates JWT secret mismatch" -ForegroundColor Yellow
    }
    
} catch {
    Write-Host "❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "This indicates Supabase configuration issue" -ForegroundColor Yellow
}

Write-Host "`nTest completed." -ForegroundColor Cyan 