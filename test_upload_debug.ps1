Write-Host "UPLOAD DEBUG TEST" -ForegroundColor Yellow
Write-Host "=================" -ForegroundColor Yellow

$baseUrl = "http://localhost:8080"

# Step 1: Login
Write-Host "`n1. Login..." -ForegroundColor Cyan
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$body = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method POST -Headers $headers -Body $body
    Write-Host "✅ Login successful!" -ForegroundColor Green
    $token = $response.token
} catch {
    Write-Host "❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Step 2: Create simple test file
Write-Host "`n2. Creating test file..." -ForegroundColor Cyan
$testContent = "Simple test content for upload debug"
$testFile = "upload_debug_test.txt"
$testContent | Out-File -FilePath $testFile -Encoding UTF8
Write-Host "✅ Test file created" -ForegroundColor Green

# Step 3: Prepare upload with minimal data
Write-Host "`n3. Testing upload..." -ForegroundColor Cyan
try {
    $boundary = [System.Guid]::NewGuid().ToString()
    $fileBytes = [System.IO.File]::ReadAllBytes($testFile)
    $fileContent = [System.Text.Encoding]::UTF8.GetString($fileBytes)
    
    # Minimal multipart form
    $bodyTemplate = @"
--{0}
Content-Disposition: form-data; name="file"; filename="{1}"
Content-Type: text/plain

{2}
--{0}--
"@
    
    $uploadBody = $bodyTemplate -f $boundary, $testFile, $fileContent
    $uploadHeaders = @{
        "Authorization" = "Bearer $token"
        "Content-Type" = "multipart/form-data; boundary=$boundary"
    }
    
    Write-Host "   Uploading to: $baseUrl/api/v1/documents/upload" -ForegroundColor Gray
    Write-Host "   File size: $($fileBytes.Length) bytes" -ForegroundColor Gray
    Write-Host "   Content-Type: multipart/form-data; boundary=$boundary" -ForegroundColor Gray
    
    $uploadResponse = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/upload" -Method POST -Headers $uploadHeaders -Body $uploadBody
    Write-Host "✅ Upload successful!" -ForegroundColor Green
    Write-Host "   Document ID: $($uploadResponse.id)" -ForegroundColor White
    
} catch {
    Write-Host "❌ Upload failed!" -ForegroundColor Red
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "   Status: $($_.Exception.Response.StatusCode)" -ForegroundColor Red
    Write-Host "   Status Description: $($_.Exception.Response.StatusDescription)" -ForegroundColor Red
    
    # Try to get response body
    try {
        $responseStream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($responseStream)
        $responseBody = $reader.ReadToEnd()
        Write-Host "   Response Body: $responseBody" -ForegroundColor Red
    } catch {
        Write-Host "   Could not read response body" -ForegroundColor Red
    }
}

# Cleanup
Write-Host "`n4. Cleanup..." -ForegroundColor Cyan
if (Test-Path $testFile) { 
    Remove-Item $testFile
    Write-Host "✅ Test file cleaned up" -ForegroundColor Green
}

Write-Host "`nUpload debug test complete." -ForegroundColor Yellow 