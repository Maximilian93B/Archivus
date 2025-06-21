Write-Host "DIRECT SUPABASE AUTHENTICATION TEST" -ForegroundColor Yellow
Write-Host "===================================" -ForegroundColor Yellow

# Test direct Supabase authentication
$supabaseUrl = "https://ulnisgaeijkspqambdlh.supabase.co"
$supabaseAnonKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InVsbmlzZ2FlaWprc3BxYW1iZGxoIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NDk5MzAwMzQsImV4cCI6MjA2NTUwNjAzNH0.JGVGe7eRXGKtsyMF1zInBeHsEY7BoaqWcloRr_CzkTE"

$headers = @{
    "apikey" = $supabaseAnonKey
    "Authorization" = "Bearer $supabaseAnonKey"
    "Content-Type" = "application/json"
}

$loginBody = @{
    email = "admin@testdebug123.com"
    password = "TestPassword123!"
} | ConvertTo-Json

Write-Host "`n1. Testing Direct Supabase Login..." -ForegroundColor Cyan
try {
    $response = Invoke-RestMethod -Uri "$supabaseUrl/auth/v1/token?grant_type=password" -Method POST -Body $loginBody -Headers $headers
    Write-Host "✅ Direct Supabase login successful!" -ForegroundColor Green
    Write-Host "   Access Token: $($response.access_token.Substring(0, 50))..." -ForegroundColor Gray
    Write-Host "   User ID: $($response.user.id)" -ForegroundColor Gray
    Write-Host "   Email: $($response.user.email)" -ForegroundColor Gray
    
    # Test token validation
    Write-Host "`n2. Testing Token Validation..." -ForegroundColor Cyan
    $validateHeaders = @{
        "apikey" = $supabaseAnonKey
        "Authorization" = "Bearer $($response.access_token)"
        "Content-Type" = "application/json"
    }
    
    $userResponse = Invoke-RestMethod -Uri "$supabaseUrl/auth/v1/user" -Method GET -Headers $validateHeaders
    Write-Host "✅ Token validation successful!" -ForegroundColor Green
    Write-Host "   Validated User ID: $($userResponse.id)" -ForegroundColor Gray
    Write-Host "   Validated Email: $($userResponse.email)" -ForegroundColor Gray
    
} catch {
    Write-Host "❌ Direct Supabase authentication failed: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $errorStream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($errorStream)
        $errorBody = $reader.ReadToEnd()
        Write-Host "   Error details: $errorBody" -ForegroundColor Red
    }
}

Write-Host "`nDirect Supabase test completed." -ForegroundColor Yellow 