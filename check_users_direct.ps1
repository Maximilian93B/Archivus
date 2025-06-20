Write-Host "CHECKING USERS IN SUPABASE AUTH TABLE" -ForegroundColor Yellow
Write-Host "=====================================" -ForegroundColor Yellow

# Use the service role key to query auth.users table
$supabaseUrl = "https://ulnisgaeijkspqambdlh.supabase.co"
$serviceKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InVsbmlzZ2FlaWprc3BxYW1iZGxoIiwicm9sZSI6InNlcnZpY2Vfcm9sZSIsImlhdCI6MTc0OTkzMDAzNCwiZXhwIjoyMDY1NTA2MDM0fQ.IIjdsWXUZSs6GioZw0H07lhbyrlFVqvq5pyGU_Qv8Wk"

$headers = @{
    "apikey" = $serviceKey
    "Authorization" = "Bearer $serviceKey"
    "Content-Type" = "application/json"
}

Write-Host "`n1. Checking auth.users table..." -ForegroundColor Cyan
try {
    # Query auth.users table using PostgREST
    $response = Invoke-RestMethod -Uri "$supabaseUrl/rest/v1/auth.users?select=id,email,email_confirmed_at,created_at,user_metadata" -Method GET -Headers $headers
    
    Write-Host "✅ Successfully queried auth.users table!" -ForegroundColor Green
    Write-Host "   Total users found: $($response.Count)" -ForegroundColor Gray
    
    foreach ($user in $response) {
        Write-Host "   User: $($user.email) (ID: $($user.id))" -ForegroundColor Cyan
        Write-Host "     Email Confirmed: $($user.email_confirmed_at -ne $null)" -ForegroundColor Gray
        Write-Host "     Created: $($user.created_at)" -ForegroundColor Gray
        if ($user.user_metadata) {
            Write-Host "     Metadata: $($user.user_metadata | ConvertTo-Json -Compress)" -ForegroundColor Gray
        }
        Write-Host ""
    }
    
    # Check specifically for our test user
    $testUser = $response | Where-Object { $_.email -eq "admin@testdebug123.com" }
    if ($testUser) {
        Write-Host "✅ Found our test user: admin@testdebug123.com" -ForegroundColor Green
        Write-Host "   ID: $($testUser.id)" -ForegroundColor Cyan
        Write-Host "   Email Confirmed: $($testUser.email_confirmed_at -ne $null)" -ForegroundColor Cyan
        Write-Host "   Metadata: $($testUser.user_metadata | ConvertTo-Json -Compress)" -ForegroundColor Cyan
    } else {
        Write-Host "❌ Test user admin@testdebug123.com NOT FOUND in auth.users" -ForegroundColor Red
    }
    
} catch {
    Write-Host "❌ Failed to query auth.users: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $errorStream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($errorStream)
        $errorBody = $reader.ReadToEnd()
        Write-Host "   Error details: $errorBody" -ForegroundColor Red
    }
}

Write-Host "`nUser check completed." -ForegroundColor Yellow 