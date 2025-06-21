# Test Admin Login
$headers = @{
    "X-Tenant-Subdomain" = "testco"
    "Content-Type" = "application/json"
}

$body = @{
    email = "admin@testco.com"
    password = "SecurePass123!"
} | ConvertTo-Json

Write-Host "Testing admin login..."
Write-Host "Headers: $($headers | ConvertTo-Json)"
Write-Host "Body: $body"

try {
    $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/auth/login" -Method POST -Headers $headers -Body $body
    Write-Host "SUCCESS: Login successful"
    Write-Host "Response: $($response | ConvertTo-Json -Depth 5)"
} catch {
    Write-Host "ERROR: Login failed"
    Write-Host "Status Code: $($_.Exception.Response.StatusCode)"
    Write-Host "Status Description: $($_.Exception.Response.StatusDescription)"
    
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $errorBody = $reader.ReadToEnd()
        Write-Host "Error Body: $errorBody"
    }
} 