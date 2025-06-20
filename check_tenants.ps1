# Check what tenants might exist
$testSubdomains = @('test', 'demo', 'admin', 'default', 'archivus')

foreach ($subdomain in $testSubdomains) {
    $headers = @{
        'Content-Type' = 'application/json'
        'X-Tenant-Subdomain' = $subdomain
    }
    
    $body = @{
        email = "test@$subdomain.com"
        password = 'TestPass123!'
        first_name = 'Test'
        last_name = 'User'
    } | ConvertTo-Json
    
    Write-Host "Testing subdomain: $subdomain" -ForegroundColor Yellow
    
    try {
        $response = Invoke-WebRequest -Uri 'http://localhost:8080/api/v1/auth/register' -Method POST -Body $body -Headers $headers -UseBasicParsing -ErrorAction Stop
        Write-Host "✅ Tenant '$subdomain' exists and registration worked!" -ForegroundColor Green
        break
    } catch {
        if ($_.Exception.Message -like "*tenant not found*" -or $_.Exception.Message -like "*Invalid tenant*") {
            Write-Host "❌ Tenant '$subdomain' does not exist" -ForegroundColor Red
        } else {
            Write-Host "✅ Tenant '$subdomain' exists (got different error: user may already exist)" -ForegroundColor Green
            break
        }
    }
}

Write-Host "`nIf no existing tenants found, please create the 'testcorp' tenant using the SQL provided." -ForegroundColor Cyan 