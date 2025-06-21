# Debug Tenant Creation Script
Write-Host "Testing Tenant Creation..." -ForegroundColor Yellow

$baseUrl = "http://localhost:8080/api/v1"
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$testTenant = "phase2test-$timestamp"
$testEmail = "admin@example.com"  # Use a simpler email format
$testPassword = "TestPassword123!"

$tenantBody = @{
    name = "Phase 2 Test Corp"
    subdomain = $testTenant
    subscription_tier = "enterprise"
    admin_email = $testEmail
    admin_first_name = "Test"
    admin_last_name = "Admin"
    admin_password = $testPassword
} | ConvertTo-Json

Write-Host "Request URL: $baseUrl/tenant" -ForegroundColor Gray
Write-Host "Request Body:" -ForegroundColor Gray
Write-Host $tenantBody -ForegroundColor Gray

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/tenant" -Method POST -Body $tenantBody -ContentType "application/json"
    Write-Host "✅ SUCCESS!" -ForegroundColor Green
    Write-Host "Response:" -ForegroundColor Green
    Write-Host ($response | ConvertTo-Json -Depth 5) -ForegroundColor Green
} catch {
    Write-Host "❌ FAILED!" -ForegroundColor Red
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
    
    # Try to get detailed error response
    if ($_.Exception.Response) {
        try {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $responseBody = $reader.ReadToEnd()
            Write-Host "Server Response:" -ForegroundColor Red
            Write-Host $responseBody -ForegroundColor Red
        } catch {
            Write-Host "Could not read error response" -ForegroundColor Red
        }
    }
    
    # Also check the status code
    Write-Host "Status Code: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
} 