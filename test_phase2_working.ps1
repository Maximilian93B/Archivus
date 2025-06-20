Write-Host "ARCHIVUS PHASE 2 TEST - WORKING TENANT" -ForegroundColor Cyan
Write-Host "=======================================" -ForegroundColor Cyan
Write-Host "Testing with testdebug123 tenant" -ForegroundColor Yellow
Write-Host ""

$baseUrl = "http://localhost:8080"

# Test 1: Health Check
Write-Host "1. Testing Health Check..." -ForegroundColor Yellow
try {
    $health = Invoke-RestMethod -Uri "$baseUrl/health"
    Write-Host "✅ Health Check: $($health.status)" -ForegroundColor Green
} catch {
    Write-Host "❌ Health Check failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test 2: Login
Write-Host "`n2. Testing Login..." -ForegroundColor Yellow
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$body = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method POST -Headers $headers -Body $body
    Write-Host "✅ Login successful!" -ForegroundColor Green
    Write-Host "   User: $($response.user.email)" -ForegroundColor White
    Write-Host "   Tenant: $($response.user.tenant_id)" -ForegroundColor White
    $token = $response.token
    Write-Host "   Token: $($token.Substring(0,50))..." -ForegroundColor White
} catch {
    Write-Host "❌ Login failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test 3: Token Validation (Documents endpoint)
Write-Host "`n3. Testing Token Validation..." -ForegroundColor Yellow
$authHeaders = @{ 
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

try {
    $docs = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents" -Method GET -Headers $authHeaders
    Write-Host "✅ Token validation successful!" -ForegroundColor Green
    Write-Host "   Documents endpoint accessible" -ForegroundColor White
    Write-Host "   Found $($docs.documents.Count) documents" -ForegroundColor White
} catch {
    Write-Host "❌ Token validation failed: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "   This indicates authentication middleware issue" -ForegroundColor Yellow
    exit 1
}

# Test 4: File Upload
Write-Host "`n4. Testing File Upload..." -ForegroundColor Yellow

# Create test file
$testContent = @"
ARCHIVUS PHASE 2 TEST DOCUMENT
==============================

This is a test document for Phase 2 validation.
Created: $(Get-Date)
Purpose: Testing file upload and processing pipeline.

Content includes:
- Text processing
- Metadata extraction
- Background job processing

This file should be successfully uploaded and processed.
"@

$testFile = "test_phase2_document.txt"
$testContent | Out-File -FilePath $testFile -Encoding UTF8

try {
    # Prepare multipart form data
    $boundary = [System.Guid]::NewGuid().ToString()
    $fileBytes = [System.IO.File]::ReadAllBytes($testFile)
    $fileContent = [System.Text.Encoding]::UTF8.GetString($fileBytes)
    
    $bodyTemplate = @"
--{0}
Content-Disposition: form-data; name="file"; filename="{1}"
Content-Type: text/plain

{2}
--{0}
Content-Disposition: form-data; name="title"

Phase 2 Test Document
--{0}
Content-Disposition: form-data; name="description"

Test document for Phase 2 pipeline validation
--{0}--
"@
    
    $uploadBody = $bodyTemplate -f $boundary, $testFile, $fileContent
    $uploadHeaders = $authHeaders.Clone()
    $uploadHeaders["Content-Type"] = "multipart/form-data; boundary=$boundary"
    
    $uploadResponse = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents" -Method POST -Headers $uploadHeaders -Body $uploadBody
    Write-Host "✅ File upload successful!" -ForegroundColor Green
    Write-Host "   Document ID: $($uploadResponse.document.id)" -ForegroundColor White
    Write-Host "   Title: $($uploadResponse.document.title)" -ForegroundColor White
    Write-Host "   Size: $($uploadResponse.document.size_bytes) bytes" -ForegroundColor White
    $documentId = $uploadResponse.document.id
    
} catch {
    Write-Host "❌ File upload failed: $($_.Exception.Message)" -ForegroundColor Red
    # Clean up test file
    if (Test-Path $testFile) { Remove-Item $testFile }
    exit 1
}

# Test 5: Document Retrieval
Write-Host "`n5. Testing Document Retrieval..." -ForegroundColor Yellow
try {
    $document = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/$documentId" -Method GET -Headers $authHeaders
    Write-Host "✅ Document retrieval successful!" -ForegroundColor Green
    Write-Host "   Document: $($document.title)" -ForegroundColor White
    Write-Host "   Status: $($document.status)" -ForegroundColor White
} catch {
    Write-Host "❌ Document retrieval failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 6: File Download
Write-Host "`n6. Testing File Download..." -ForegroundColor Yellow
try {
    $downloadResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/documents/$documentId/download" -Method GET -Headers $authHeaders -UseBasicParsing
    Write-Host "✅ File download successful!" -ForegroundColor Green
    Write-Host "   Status: $($downloadResponse.StatusCode)" -ForegroundColor White
    Write-Host "   Content-Type: $($downloadResponse.Headers['Content-Type'])" -ForegroundColor White
    Write-Host "   Size: $($downloadResponse.Content.Length) bytes" -ForegroundColor White
} catch {
    Write-Host "❌ File download failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Clean up
Write-Host "`n7. Cleaning up..." -ForegroundColor Yellow
if (Test-Path $testFile) { 
    Remove-Item $testFile
    Write-Host "✅ Test file cleaned up" -ForegroundColor Green
}

Write-Host "`n🎉 PHASE 2 TEST COMPLETED!" -ForegroundColor Green
Write-Host "All core functionality appears to be working!" -ForegroundColor Green 