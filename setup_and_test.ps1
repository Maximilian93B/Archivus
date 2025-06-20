# Archivus Complete Setup and Test Script
# This script will guide you through setting up a test tenant and running the full pipeline

Write-Host "ARCHIVUS COMPLETE SETUP AND TESTING" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

$baseUrl = "http://localhost:8080"

# Database connection details from .env
$dbHost = "db.ulnisgaeijkspqambdlh.supabase.co"
$dbPort = "5432"
$dbName = "postgres"
$dbUser = "postgres"
$dbPassword = "Y5HK4_.D5Cc7v.y"

function Test-SystemHealth {
    Write-Host "=== STEP 1: CHECKING SYSTEM HEALTH ===" -ForegroundColor Cyan
    
    try {
        $healthResponse = Invoke-WebRequest -Uri "$baseUrl/health" -UseBasicParsing -TimeoutSec 5
        $readyResponse = Invoke-WebRequest -Uri "$baseUrl/ready" -UseBasicParsing -TimeoutSec 5
        
        if ($healthResponse.StatusCode -eq 200 -and $readyResponse.StatusCode -eq 200) {
            Write-Host "✅ System is healthy and ready" -ForegroundColor Green
            return $true
        }
    } catch {
        Write-Host "❌ System health check failed: $($_.Exception.Message)" -ForegroundColor Red
        return $false
    }
    
    return $false
}

function Show-TenantCreationOptions {
    Write-Host "=== STEP 2: TENANT CREATION REQUIRED ===" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "The system needs a test tenant. Choose your preferred method:" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "OPTION 1: Supabase Dashboard (Recommended)" -ForegroundColor Green
    Write-Host "  1. Go to: https://supabase.com/dashboard/projects" -ForegroundColor White
    Write-Host "  2. Select your project: ulnisgaeijkspqambdlh" -ForegroundColor White
    Write-Host "  3. Go to: Table Editor > tenants" -ForegroundColor White
    Write-Host "  4. Click 'Insert row'" -ForegroundColor White
    Write-Host "  5. Fill in:" -ForegroundColor White
    Write-Host "     - name: Test Corporation" -ForegroundColor Gray
    Write-Host "     - subdomain: testcorp" -ForegroundColor Gray
    Write-Host "     - subscription_tier: starter" -ForegroundColor Gray
    Write-Host "     - is_active: true" -ForegroundColor Gray
    Write-Host "     - storage_quota: 5368709120" -ForegroundColor Gray
    Write-Host "     - api_quota: 1000" -ForegroundColor Gray
    Write-Host ""
    
    Write-Host "OPTION 2: psql Command Line" -ForegroundColor Green
    Write-Host "  Install PostgreSQL client, then run:" -ForegroundColor White
    Write-Host "  psql -h $dbHost -p $dbPort -U $dbUser -d $dbName -f create_test_tenant.sql" -ForegroundColor Gray
    Write-Host ""
    
    Write-Host "OPTION 3: Any PostgreSQL Client (pgAdmin, DBeaver, etc.)" -ForegroundColor Green
    Write-Host "  Connection details:" -ForegroundColor White
    Write-Host "    Host: $dbHost" -ForegroundColor Gray
    Write-Host "    Port: $dbPort" -ForegroundColor Gray
    Write-Host "    Database: $dbName" -ForegroundColor Gray
    Write-Host "    Username: $dbUser" -ForegroundColor Gray
    Write-Host "    Password: $dbPassword" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  Then run the SQL from: create_test_tenant.sql" -ForegroundColor Gray
    Write-Host ""
    
    Write-Host "OPTION 4: Manual SQL (Copy and Paste)" -ForegroundColor Green
    Write-Host "Copy this SQL and run it in any PostgreSQL client:" -ForegroundColor White
    Write-Host ""
    Write-Host "INSERT INTO tenants (id, name, subdomain, subscription_tier, storage_quota, api_quota, settings, is_active, created_at, updated_at)" -ForegroundColor Yellow
    Write-Host "VALUES (gen_random_uuid(), 'Test Corporation', 'testcorp', 'starter', 5368709120, 1000, '{}', true, now(), now())" -ForegroundColor Yellow
    Write-Host "ON CONFLICT (subdomain) DO NOTHING;" -ForegroundColor Yellow
    Write-Host ""
}

function Test-TenantExists {
    Write-Host "=== CHECKING IF TENANT EXISTS ===" -ForegroundColor Cyan
    
    $headers = @{
        "Content-Type" = "application/json"
        "X-Tenant-Subdomain" = "testcorp"
    }
    
    $testBody = @{
        email = "test.existence.check@domain.com"
        password = "TestPass123!"
        first_name = "Test"
        last_name = "Check"
    } | ConvertTo-Json
    
    try {
        $response = Invoke-WebRequest -Uri "$baseUrl/api/v1/auth/register" -Method "POST" -Body $testBody -Headers $headers -UseBasicParsing -ErrorAction Stop
        # If we get here, either user was created or some other error occurred
        # This means tenant exists (would fail earlier if tenant didn't exist)
        Write-Host "✅ Tenant 'testcorp' exists and is accessible" -ForegroundColor Green
        return $true
    } catch {
        $errorResponse = $_.Exception.Response
        if ($errorResponse -and $errorResponse.StatusCode -eq 400) {
            # Try to read the error message
            try {
                $stream = $errorResponse.GetResponseStream()
                $reader = New-Object System.IO.StreamReader($stream)
                $errorContent = $reader.ReadToEnd()
                
                if ($errorContent -like "*tenant not found*" -or $errorContent -like "*Invalid tenant*") {
                    Write-Host "❌ Tenant 'testcorp' does not exist" -ForegroundColor Red
                    return $false
                } else {
                    Write-Host "✅ Tenant 'testcorp' exists (got different 400 error: user may already exist)" -ForegroundColor Green
                    return $true
                }
            } catch {
                Write-Host "⚠️ Could not determine tenant status" -ForegroundColor Yellow
                return $false
            }
        } else {
            Write-Host "⚠️ Unexpected error checking tenant: $($_.Exception.Message)" -ForegroundColor Yellow
            return $false
        }
    }
}

function Run-PipelineTest {
    Write-Host "=== STEP 3: RUNNING COMPLETE PIPELINE TEST ===" -ForegroundColor Cyan
    Write-Host ""
    
    try {
        & "./test_proper_pipeline.ps1"
        return $true
    } catch {
        Write-Host "❌ Pipeline test failed: $($_.Exception.Message)" -ForegroundColor Red
        return $false
    }
}

# Main execution
Write-Host "Starting Archivus setup and testing process..." -ForegroundColor Green
Write-Host ""

# Step 1: Check system health
if (-not (Test-SystemHealth)) {
    Write-Host "Please ensure the Archivus server is running and try again." -ForegroundColor Red
    Write-Host "Start the server with: `$env:CGO_ENABLED = `"0`"; go run cmd/server/main.go" -ForegroundColor Yellow
    exit 1
}

# Step 2: Check if tenant exists
if (Test-TenantExists) {
    Write-Host "✅ Tenant setup complete - proceeding to pipeline test" -ForegroundColor Green
    Write-Host ""
    
    # Step 3: Run pipeline test
    if (Run-PipelineTest) {
        Write-Host ""
        Write-Host "🎉 ARCHIVUS SETUP AND TESTING COMPLETE!" -ForegroundColor Green
        Write-Host "====================================" -ForegroundColor Green
        Write-Host "Your Archivus system is fully operational and ready for development!" -ForegroundColor White
    } else {
        Write-Host "⚠️ Setup complete but pipeline test had issues. Check the output above." -ForegroundColor Yellow
    }
} else {
    Write-Host "Tenant creation required. Please follow the instructions below:" -ForegroundColor Yellow
    Write-Host ""
    Show-TenantCreationOptions
    Write-Host ""
    Write-Host "After creating the tenant, run this script again:" -ForegroundColor Green
    Write-Host "./setup_and_test.ps1" -ForegroundColor Cyan
}

Write-Host ""
Write-Host "Setup script completed!" -ForegroundColor Cyan 