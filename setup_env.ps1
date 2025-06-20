Write-Host "🔧 ARCHIVUS ENVIRONMENT SETUP" -ForegroundColor Cyan
Write-Host "==============================" -ForegroundColor Cyan
Write-Host ""

Write-Host "This script will help you create the .env file with proper Supabase configuration." -ForegroundColor Yellow
Write-Host ""

# Check if .env already exists
if (Test-Path ".env") {
    Write-Host "⚠️ .env file already exists. Creating backup..." -ForegroundColor Yellow
    Copy-Item ".env" ".env.backup"
    Write-Host "✅ Backup created: .env.backup" -ForegroundColor Green
}

Write-Host "📋 TO GET YOUR SUPABASE CREDENTIALS:" -ForegroundColor Magenta
Write-Host "=====================================" -ForegroundColor Magenta
Write-Host "1. Go to: https://supabase.com/dashboard/projects" -ForegroundColor White
Write-Host "2. Select your project: ulnisgaeijkspqambdlh" -ForegroundColor White
Write-Host "3. Go to Settings → API" -ForegroundColor White
Write-Host "4. Copy the following values:" -ForegroundColor White
Write-Host "   - Project URL" -ForegroundColor Gray
Write-Host "   - anon/public key" -ForegroundColor Gray
Write-Host "   - service_role key" -ForegroundColor Gray
Write-Host "   - JWT Secret" -ForegroundColor Gray
Write-Host ""

# Prompt for Supabase credentials
Write-Host "📝 ENTER YOUR SUPABASE CREDENTIALS:" -ForegroundColor Cyan
Write-Host "===================================" -ForegroundColor Cyan

$supabaseUrl = Read-Host "Enter your Supabase URL (e.g., https://your-project.supabase.co)"
$supabaseAnonKey = Read-Host "Enter your Supabase anon/public key"
$supabaseServiceKey = Read-Host "Enter your Supabase service_role key"
$supabaseJwtSecret = Read-Host "Enter your Supabase JWT Secret"

# Create .env file content
$envContent = @"
# Development Environment Configuration
ENVIRONMENT=development

# Server Configuration
HOST=localhost
PORT=8080
ALLOWED_ORIGINS="http://localhost:3000,http://localhost:8080"

# Database Configuration
DATABASE_URL="postgres://postgres:postgres@localhost:5432/archivus_dev?sslmode=disable"

# Redis Configuration
REDIS_URL="redis://localhost:6379"

# JWT Configuration (Using Supabase JWT Secret)
JWT_SECRET="$supabaseJwtSecret"

# Supabase Configuration
SUPABASE_URL="$supabaseUrl"
SUPABASE_API_KEY="$supabaseAnonKey"
SUPABASE_SERVICE_KEY="$supabaseServiceKey"
SUPABASE_JWT_SECRET="$supabaseJwtSecret"
SUPABASE_BUCKET="documents"

# Storage Configuration
STORAGE_TYPE="local"
STORAGE_PATH="./uploads"

# AI Configuration
ENABLE_AI_PROCESSING="false"
OPENAI_API_KEY=""

# Feature Flags
ENABLE_OCR="false"
ENABLE_WEBHOOKS="false"

# File Upload Limits
MAX_FILE_SIZE="104857600"
ALLOWED_FILE_TYPES="pdf,doc,docx,txt,jpg,jpeg,png,gif"

# Rate Limiting
RATE_LIMIT_REQUESTS="100"
RATE_LIMIT_WINDOW="60s"
"@

# Write .env file
Set-Content -Path ".env" -Value $envContent

Write-Host ""
Write-Host "✅ .env file created successfully!" -ForegroundColor Green
Write-Host "🔄 You need to restart your Docker services now:" -ForegroundColor Yellow
Write-Host "   docker-compose down" -ForegroundColor White
Write-Host "   docker-compose up -d" -ForegroundColor White
Write-Host ""
Write-Host "🧪 Then run Phase 2 tests with:" -ForegroundColor Yellow
Write-Host "   ./test_jwt_fix.ps1" -ForegroundColor White 