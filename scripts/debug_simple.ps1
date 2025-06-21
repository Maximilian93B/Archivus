# Simple Debug Test - Lowercase Alphanumeric Only
Write-Host "Simple Debug Test..." -ForegroundColor Yellow

$baseUrl = "http://localhost:8080/api/v1"

# Simple, short, lowercase alphanumeric subdomain
$testTenant = "test123"
$testEmail = "admin@test123.com"
$testPassword = "Password123!"

Write-Host "Using simple subdomain: $testTenant" -ForegroundColor Cyan

$tenantBody = @{
    name = "TestCorp"
    subdomain = $testTenant
    admin_email = $testEmail
    admin_first_name = "Admin"
    admin_last_name = "User"
    admin_password = $testPassword
} | ConvertTo-Json

Write-Host "Request Body:" -ForegroundColor Gray
Write-Host $tenantBody -ForegroundColor Gray

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/tenant" -Method POST -Body $tenantBody -ContentType "application/json"
    Write-Host "🎉 SUCCESS!" -ForegroundColor Green
    Write-Host ($response | ConvertTo-Json) -ForegroundColor Green
} catch {
    Write-Host "❌ FAILED!" -ForegroundColor Red
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
    
    # Try to check if the server logs show anything useful
    Write-Host "`nTesting if other endpoints work..." -ForegroundColor Yellow
    
    # Test a simple GET endpoint that should work
    try {
        $healthResponse = Invoke-RestMethod -Uri "http://localhost:8080/health" -UseBasicParsing
        Write-Host "✅ Health endpoint works: $($healthResponse.status)" -ForegroundColor Green
    } catch {
        Write-Host "❌ Even health endpoint fails" -ForegroundColor Red
    }
    
    # Test if we can hit the auth endpoint with a simple request
    try {
        $authTest = Invoke-RestMethod -Uri "$baseUrl/auth/validate" -Method GET
    } catch {
        Write-Host "Auth endpoint test: $($_.Exception.Message)" -ForegroundColor Gray
    }
} 