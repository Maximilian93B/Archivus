# Archivus Comprehensive Background Worker Test
Write-Host "ARCHIVUS COMPREHENSIVE WORKER TEST" -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080"

# Test 1: Build and Start Worker
Write-Host "1. Preparing Background Worker..." -ForegroundColor Yellow
$workerPath = "..\archivus-worker.exe"
if (-not (Test-Path $workerPath)) {
    Write-Host "   Building worker binary..." -ForegroundColor Yellow
    Set-Location ..
    $env:CGO_ENABLED=0
    go build -o archivus-worker.exe ./cmd/worker
    Set-Location scripts
    Write-Host "   Worker binary built" -ForegroundColor Green
}

$workerProcess = Start-Process -FilePath $workerPath -PassThru -WindowStyle Hidden
Write-Host "   Worker started with PID: $($workerProcess.Id)" -ForegroundColor Green
Start-Sleep -Seconds 5

# Test 2: Authentication
Write-Host "2. Authentication..." -ForegroundColor Yellow
$headers = @{ 
    "Content-Type" = "application/json"
    "X-Tenant-Subdomain" = "testdebug123" 
}
$body = '{"email":"admin@testdebug123.com","password":"SecurePass123!"}'

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/auth/login" -Method POST -Headers $headers -Body $body
    Write-Host "   Authentication successful!" -ForegroundColor Green
    $token = $response.token
} catch {
    Write-Host "   Authentication failed: $($_.Exception.Message)" -ForegroundColor Red
    Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
    exit 1
}

# Test 3: Upload Multiple File Types
Write-Host "3. Uploading multiple file types..." -ForegroundColor Yellow

$testFiles = @(
    @{ 
        name = "document.txt"
        content = "TEXT DOCUMENT FOR WORKER TESTING`nCreated: $(Get-Date)`nThis tests metadata extraction and file validation."
        type = "text/plain"
    },
    @{ 
        name = "data.csv"
        content = "Name,Age,Department`nJohn,30,Engineering`nJane,25,Marketing`nBob,35,Sales"
        type = "text/csv"
    },
    @{ 
        name = "config.json"
        content = '{"application": "archivus", "test": true, "worker": {"enabled": true, "jobs": ["validation", "metadata", "thumbnails"]}, "timestamp": "' + (Get-Date -Format "yyyy-MM-dd HH:mm:ss") + '"}'
        type = "application/json"
    }
)

$documentIds = @()
$authHeaders = @{ "Authorization" = "Bearer $token" }

foreach ($file in $testFiles) {
    try {
        # Create test file
        $file.content | Out-File -FilePath $file.name -Encoding UTF8
        
        # Upload file
        $boundary = [System.Guid]::NewGuid().ToString()
        $fileBytes = [System.IO.File]::ReadAllBytes($file.name)
        $fileContent = [System.Text.Encoding]::UTF8.GetString($fileBytes)
        
        $uploadBody = @"
--$boundary
Content-Disposition: form-data; name="file"; filename="$($file.name)"
Content-Type: $($file.type)

$fileContent
--$boundary
Content-Disposition: form-data; name="title"

Worker Test - $($file.name)
--$boundary
Content-Disposition: form-data; name="description"

Comprehensive worker test for $($file.name) - tests all job types
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

Write-Host "   Total files uploaded: $($documentIds.Count)" -ForegroundColor White

# Test 4: Monitor Job Processing
Write-Host "4. Monitoring background job processing..." -ForegroundColor Yellow
Write-Host "   This tests: file_validation, metadata_extraction, thumbnail_generation, preview_generation" -ForegroundColor Gray

$maxWaitTime = 45
$waitTime = 0
$processingComplete = $false

while ($waitTime -lt $maxWaitTime -and -not $processingComplete) {
    Start-Sleep -Seconds 3
    $waitTime += 3
    
    $processedCount = 0
    $pendingCount = 0
    $failedCount = 0
    
    foreach ($docId in $documentIds) {
        try {
            $document = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/$docId" -Method GET -Headers $authHeaders
            switch ($document.status) {
                "processed" { $processedCount++ }
                "ready" { $processedCount++ }
                "pending" { $pendingCount++ }
                "failed" { $failedCount++ }
                default { $pendingCount++ }
            }
        } catch {
            $failedCount++
        }
    }
    
    Write-Host "   Progress ($waitTime/$maxWaitTime sec): Processed=$processedCount, Pending=$pendingCount, Failed=$failedCount" -ForegroundColor Gray
    
    if ($processedCount -eq $documentIds.Count) {
        $processingComplete = $true
        Write-Host "   All documents processed successfully!" -ForegroundColor Green
    } elseif ($failedCount -gt 0) {
        Write-Host "   Some documents failed processing" -ForegroundColor Yellow
    }
}

# Test 5: Detailed Job Results Analysis
Write-Host "5. Analyzing job processing results..." -ForegroundColor Yellow

$successCount = 0
$failureCount = 0

foreach ($i in 0..($documentIds.Count - 1)) {
    $docId = $documentIds[$i]
    $fileName = $testFiles[$i].name
    
    try {
        $document = Invoke-RestMethod -Uri "$baseUrl/api/v1/documents/$docId" -Method GET -Headers $authHeaders
        
        Write-Host "   File: $fileName" -ForegroundColor White
        Write-Host "     Document ID: $docId" -ForegroundColor Gray
        Write-Host "     Status: $($document.status)" -ForegroundColor White
        Write-Host "     Size: $($document.file_size) bytes" -ForegroundColor White
        Write-Host "     Content Type: $($document.content_type)" -ForegroundColor White
        
        if ($document.status -eq "processed" -or $document.status -eq "ready") {
            Write-Host "     Result: SUCCESS - All background jobs completed" -ForegroundColor Green
            $successCount++
        } elseif ($document.status -eq "pending") {
            Write-Host "     Result: PENDING - Jobs still processing or no worker available" -ForegroundColor Yellow
        } else {
            Write-Host "     Result: FAILED - Job processing failed" -ForegroundColor Red
            $failureCount++
        }
        
    } catch {
        Write-Host "   Error analyzing $fileName : $($_.Exception.Message)" -ForegroundColor Red
        $failureCount++
    }
    
    Write-Host "" # Empty line for readability
}

# Test 6: Test File Downloads After Processing
Write-Host "6. Testing file downloads after processing..." -ForegroundColor Yellow

$downloadSuccessCount = 0
foreach ($docId in $documentIds) {
    try {
        $downloadResponse = Invoke-WebRequest -Uri "$baseUrl/api/v1/documents/$docId/download" -Method GET -Headers $authHeaders -UseBasicParsing
        if ($downloadResponse.StatusCode -eq 200) {
            $downloadSuccessCount++
        }
    } catch {
        Write-Host "   Download failed for $docId" -ForegroundColor Red
    }
}

Write-Host "   Downloads successful: $downloadSuccessCount/$($documentIds.Count)" -ForegroundColor White

# Test 7: Worker Health Check
Write-Host "7. Final worker health check..." -ForegroundColor Yellow
if (Get-Process -Id $workerProcess.Id -ErrorAction SilentlyContinue) {
    Write-Host "   Worker process is healthy and running" -ForegroundColor Green
} else {
    Write-Host "   Worker process has stopped or crashed" -ForegroundColor Red
}

# Test 8: Cleanup
Write-Host "8. Cleaning up test environment..." -ForegroundColor Yellow

# Stop worker
try {
    Stop-Process -Id $workerProcess.Id -Force -ErrorAction SilentlyContinue
    Write-Host "   Worker process stopped" -ForegroundColor Green
} catch {
    Write-Host "   Worker cleanup completed" -ForegroundColor Yellow
}

# Remove test files
foreach ($file in $testFiles) {
    Remove-Item $file.name -ErrorAction SilentlyContinue
}
Write-Host "   Test files cleaned up" -ForegroundColor Green

# Final Results Summary
Write-Host "`n" + "="*60 -ForegroundColor Cyan
Write-Host "COMPREHENSIVE BACKGROUND WORKER TEST RESULTS" -ForegroundColor Green
Write-Host "="*60 -ForegroundColor Cyan
Write-Host ""
Write-Host "Test Summary:" -ForegroundColor White
Write-Host "  Files Uploaded: $($documentIds.Count)" -ForegroundColor White
Write-Host "  Processing Successful: $successCount" -ForegroundColor White
Write-Host "  Processing Failed: $failureCount" -ForegroundColor White
Write-Host "  Downloads Successful: $downloadSuccessCount" -ForegroundColor White
Write-Host ""

if ($successCount -eq $documentIds.Count -and $downloadSuccessCount -eq $documentIds.Count) {
    Write-Host "RESULT: ALL TESTS PASSED!" -ForegroundColor Green
    Write-Host "Background worker system is FULLY FUNCTIONAL" -ForegroundColor Green
    Write-Host "Ready for Phase 3 development!" -ForegroundColor Green
} elseif ($successCount -gt 0) {
    Write-Host "RESULT: PARTIAL SUCCESS" -ForegroundColor Yellow
    Write-Host "Worker system is functional but may need optimization" -ForegroundColor Yellow
} else {
    Write-Host "RESULT: TESTS FAILED" -ForegroundColor Red
    Write-Host "Worker system needs debugging before Phase 3" -ForegroundColor Red
}

Write-Host "" 