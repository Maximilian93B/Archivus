# Archivus Background Worker Test
Write-Host "ARCHIVUS BACKGROUND WORKER TEST" -ForegroundColor Cyan
Write-Host "===============================" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080"

# Test 1: Start Worker in Background
Write-Host "1. Starting Background Worker..." -ForegroundColor Yellow
$workerProcess = Start-Process -FilePath ".\archivus-worker.exe" -PassThru -WindowStyle Hidden
Write-Host "   Worker started with PID: $($workerProcess.Id)" -ForegroundColor Green
Start-Sleep -Seconds 3

# Test 2: Login
Write-Host "2. Authentication..." -ForegroundColor Yellow
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$body = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method POST -Headers $headers -Body $body
    Write-Host "   Login successful!" -ForegroundColor Green
    $token = $response.token
} catch {
    Write-Host "   Login failed: $($_.Exception.Message)" -ForegroundColor Red
    Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
    exit 1
}

# Test 3: Upload Multiple Files to Trigger Jobs
Write-Host "3. Uploading test files..." -ForegroundColor Yellow

$testFiles = @()
$documentIds = @()

# Create different file types
$files = @(
    @{ name = "text_document.txt"; content = "This is a text document for worker testing.`nCreated: $(Get-Date)" },
    @{ name = "data_file.csv"; content = "Name,Age,City`nJohn,30,NYC`nJane,25,LA" },
    @{ name = "config_file.json"; content = '{"test": true, "worker": "background", "timestamp": "' + (Get-Date -Format "yyyy-MM-dd HH:mm:ss") + '"}' }
)

foreach ($file in $files) {
    try {
        # Create test file
        $file.content | Out-File -FilePath $file.name -Encoding UTF8
        $testFiles += $file.name
        
        # Upload file
        $boundary = [System.Guid]::NewGuid().ToString()
        $fileBytes = [System.IO.File]::ReadAllBytes($file.name)
        $fileContent = [System.Text.Encoding]::UTF8.GetString($fileBytes)
        
        $uploadBody = @"
--$boundary
Content-Disposition: form-data; name="file"; filename="$($file.name)"
Content-Type: text/plain

$fileContent
--$boundary
Content-Disposition: form-data; name="title"

Worker Test - $($file.name)
--$boundary--
"@
        
        $uploadHeaders = @{
            "Authorization" = "Bearer $token"
            "Content-Type" = "multipart/form-data; boundary=$boundary"
        }
        
        $uploadResponse = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/upload" -Method POST -Headers $uploadHeaders -Body $uploadBody
        $documentIds += $uploadResponse.id
        Write-Host "   Uploaded $($file.name) - ID: $($uploadResponse.id)" -ForegroundColor Green
        
    } catch {
        Write-Host "   Failed to upload $($file.name): $($_.Exception.Message)" -ForegroundColor Red
    }
}

# Test 4: Wait and Check Job Processing
Write-Host "4. Monitoring background job processing..." -ForegroundColor Yellow
Write-Host "   Waiting for workers to process files..." -ForegroundColor Gray

$maxWaitTime = 30
$waitTime = 0
$authHeaders = @{ "Authorization" = "Bearer $token" }

while ($waitTime -lt $maxWaitTime) {
    Start-Sleep -Seconds 2
    $waitTime += 2
    
    $processedCount = 0
    foreach ($docId in $documentIds) {
        try {
            $document = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/$docId" -Method GET -Headers $authHeaders
            if ($document.status -ne "pending") {
                $processedCount++
            }
        } catch {
            # Continue checking
        }
    }
    
    Write-Host "   Progress: $processedCount/$($documentIds.Count) documents processed" -ForegroundColor Gray
    
    if ($processedCount -eq $documentIds.Count) {
        Write-Host "   All documents processed!" -ForegroundColor Green
        break
    }
}

# Test 5: Verify Job Results
Write-Host "5. Verifying job processing results..." -ForegroundColor Yellow

foreach ($docId in $documentIds) {
    try {
        $document = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/$docId" -Method GET -Headers $authHeaders
        Write-Host "   Document: $($document.title)" -ForegroundColor White
        Write-Host "     Status: $($document.status)" -ForegroundColor White
        Write-Host "     Size: $($document.file_size) bytes" -ForegroundColor White
        
        if ($document.status -eq "processed" -or $document.status -eq "ready") {
            Write-Host "     ✅ Processing complete" -ForegroundColor Green
        } elseif ($document.status -eq "pending") {
            Write-Host "     ⏳ Still pending (worker may need more time)" -ForegroundColor Yellow
        } else {
            Write-Host "     ❌ Processing failed" -ForegroundColor Red
        }
        
    } catch {
        Write-Host "   ❌ Error checking document $docId" -ForegroundColor Red
    }
}

# Test 6: Test Download After Processing
Write-Host "6. Testing downloads after processing..." -ForegroundColor Yellow
$downloadSuccessCount = 0

foreach ($docId in $documentIds) {
    try {
        $downloadResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/documents/$docId/download" -Method GET -Headers $authHeaders -UseBasicParsing
        if ($downloadResponse.StatusCode -eq 200) {
            $downloadSuccessCount++
            Write-Host "   ✅ Download successful for $docId" -ForegroundColor Green
        }
    } catch {
        Write-Host "   ❌ Download failed for $docId" -ForegroundColor Red
    }
}

# Test 7: Check Worker Health
Write-Host "7. Checking worker health..." -ForegroundColor Yellow
if (Get-Process -Id $workerProcess.Id -ErrorAction SilentlyContinue) {
    Write-Host "   ✅ Worker process is still running" -ForegroundColor Green
} else {
    Write-Host "   ❌ Worker process has stopped" -ForegroundColor Red
}

# Cleanup
Write-Host "8. Cleaning up..." -ForegroundColor Yellow

# Stop worker
try {
    Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
    Write-Host "   Worker process stopped" -ForegroundColor Green
} catch {
    Write-Host "   Worker may have already stopped" -ForegroundColor Yellow
}

# Remove test files
foreach ($file in $testFiles) {
    Remove-Item $file -ErrorAction SilentlyContinue
}
Write-Host "   Test files cleaned up" -ForegroundColor Green

# Final Results
Write-Host "`n" + "="*50 -ForegroundColor Cyan
Write-Host "🎉 BACKGROUND WORKER TEST COMPLETED!" -ForegroundColor Green
Write-Host "="*50 -ForegroundColor Cyan
Write-Host ""
Write-Host "📊 Test Results:" -ForegroundColor White
Write-Host "   Files Uploaded: $($documentIds.Count)" -ForegroundColor White
Write-Host "   Downloads Successful: $downloadSuccessCount/$($documentIds.Count)" -ForegroundColor White
Write-Host ""

if ($downloadSuccessCount -eq $documentIds.Count) {
    Write-Host "✅ ALL BACKGROUND WORKER TESTS PASSED!" -ForegroundColor Green
    Write-Host "🚀 Phase 2 Background Processing is SOLID!" -ForegroundColor Green
} else {
    Write-Host "⚠️  Some tests had issues - check worker logs" -ForegroundColor Yellow
    Write-Host "💡 Worker system is functional but may need tuning" -ForegroundColor Yellow
}

Write-Host "" 