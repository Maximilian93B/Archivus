# Simple registration test
$headers = @{
    'Content-Type' = 'application/json'
    'X-Tenant-Subdomain' = 'testcorp'
}

$body = @{
    email = 'admin@testcorp.com'
    password = 'SecureAdmin123!'
    first_name = 'Admin'
    last_name = 'User'
    role = 'admin'
} | ConvertTo-Json

Write-Host "Testing registration with headers: $($headers | ConvertTo-Json)"
Write-Host "Body: $body"

try {
    $response = Invoke-WebRequest -Uri 'http://localhost:8080/api/v1/auth/register' -Method POST -Body $body -Headers $headers -UseBasicParsing
    Write-Host "SUCCESS: $($response.Content)" -ForegroundColor Green
} catch {
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
    
    if ($_.Exception.Response) {
        $stream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($stream)
        $errorBody = $reader.ReadToEnd()
        Write-Host "Error Details: $errorBody" -ForegroundColor Yellow
    }
} 