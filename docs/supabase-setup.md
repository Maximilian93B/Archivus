# Archivus + Supabase Integration Guide

## 🚀 Why Supabase is Perfect for Archivus

Supabase provides **exactly** what Archivus needs:
- ✅ **PostgreSQL 16+** with `pgvector` extension support
- ✅ **Built-in Authentication** (can complement our JWT system)
- ✅ **Storage API** for document files
- ✅ **Real-time subscriptions** for live updates
- ✅ **Row Level Security** for multi-tenancy
- ✅ **Auto-generated REST API** (backup to our custom API)

## 📋 Current Compatibility Status

| Feature | Archivus Implementation | Supabase Support | Status |
|---------|------------------------|------------------|---------|
| PostgreSQL | ✅ GORM + pgx driver | ✅ Native | **Perfect** |
| pgvector | ✅ AI embeddings | ✅ Extension enabled | **Perfect** |
| uuid-ossp | ✅ UUID generation | ✅ Auto-enabled | **Perfect** |
| Multi-tenancy | ✅ Custom tenant isolation | ✅ RLS policies | **Enhanced** |
| Authentication | ✅ Custom JWT | ✅ Supabase Auth | **Flexible** |
| File Storage | ✅ Interface-based | ✅ Storage API | **Perfect** |
| Real-time | ❌ Not implemented | ✅ Built-in | **Upgrade** |

## 🔧 Supabase Setup for Archivus

### 1. Create Supabase Project

```bash
# Go to https://supabase.com/dashboard
# Create new project: "archivus-production"
# Note your project URL and anon key
```

### 2. Enable Required Extensions

```sql
-- Run in Supabase SQL Editor
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";
```

### 3. Update Environment Configuration

```bash
# .env.production
ENVIRONMENT=production

# Supabase Database Connection
DATABASE_URL=postgresql://postgres:[YOUR-PASSWORD]@db.[PROJECT-REF].supabase.co:5432/postgres?sslmode=require

# Supabase Configuration
SUPABASE_URL=https://[PROJECT-REF].supabase.co
SUPABASE_ANON_KEY=[YOUR-ANON-KEY]
SUPABASE_SERVICE_KEY=[YOUR-SERVICE-KEY]

# Existing Archivus Config
JWT_SECRET=your-production-jwt-secret
STORAGE_TYPE=supabase
ENABLE_AI_PROCESSING=true
```

## 🏗️ Architecture Integration Options

### Option 1: Hybrid Authentication (Recommended)

**Use both Supabase Auth + Custom JWT for maximum flexibility**

```go
// Custom middleware that supports both
func (h *AuthHandler) FlexibleAuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Try Supabase JWT first
        if supabaseToken := c.GetHeader("Authorization"); supabaseToken != "" {
            if user, err := h.validateSupabaseToken(supabaseToken); err == nil {
                SetUserContext(c, user)
                c.Next()
                return
            }
        }
        
        // Fallback to custom JWT (for API keys, service tokens)
        h.AuthMiddleware()(c)
    }
}
```

### Option 2: Full Supabase Auth Migration

**Replace custom auth entirely with Supabase**

```go
// Update UserService to use Supabase Auth
type SupabaseAuthService struct {
    client *supabase.Client
}

func (s *SupabaseAuthService) Login(email, password string) (*LoginResult, error) {
    resp, err := s.client.Auth.SignInWithEmailPassword(email, password)
    // Convert to our UserContext format
    return convertSupabaseUser(resp)
}
```

## 🗃️ Row Level Security for Multi-Tenancy

### Enable RLS on All Tables

```sql
-- Enable RLS for all Archivus tables
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE folders ENABLE ROW LEVEL SECURITY;
ALTER TABLE tags ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_processing_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE shares ENABLE ROW LEVEL SECURITY;

-- Create RLS policies
CREATE POLICY "Users can only access their tenant data" ON documents
    FOR ALL USING (tenant_id = auth.jwt() ->> 'tenant_id'::text::uuid);

CREATE POLICY "Users can only access their tenant users" ON users
    FOR ALL USING (tenant_id = auth.jwt() ->> 'tenant_id'::text::uuid);

-- Admin policies
CREATE POLICY "Tenant admins can manage tenant" ON tenants
    FOR ALL USING (
        id = auth.jwt() ->> 'tenant_id'::text::uuid 
        AND auth.jwt() ->> 'role' = 'admin'
    );
```

## 📁 Supabase Storage Integration

### Update Storage Service

```go
// internal/infrastructure/storage/supabase_storage.go
type SupabaseStorageService struct {
    client *supabase.Client
    bucket string
}

func (s *SupabaseStorageService) Store(ctx context.Context, params StorageParams) (string, error) {
    // Upload to Supabase Storage
    path := fmt.Sprintf("%s/documents/%s", params.TenantID, params.Filename)
    
    _, err := s.client.Storage.
        From(s.bucket).
        Upload(path, params.FileReader, supabase.FileOptions{
            ContentType: params.ContentType,
        })
    
    if err != nil {
        return "", err
    }
    
    return path, nil
}

func (s *SupabaseStorageService) GeneratePresignedURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
    return s.client.Storage.
        From(s.bucket).
        CreateSignedURL(path, int(expiry.Seconds()))
}
```

## 🔄 Real-time Features with Supabase

### Enable Real-time Document Updates

```go
// Add real-time document collaboration
type RealtimeService struct {
    client *supabase.Client
}

func (r *RealtimeService) SubscribeToDocumentChanges(tenantID uuid.UUID, callback func(*models.Document)) {
    r.client.Realtime.
        From("documents").
        On("*").
        Filter("tenant_id", "eq", tenantID.String()).
        Subscribe(func(payload supabase.Payload) {
            // Convert to our Document model and call callback
            doc := convertPayloadToDocument(payload)
            callback(doc)
        })
}
```

## 🚀 Deployment to Supabase

### 1. Database Migration

```bash
# Run our existing migrations on Supabase
export DATABASE_URL="postgresql://postgres:[PASSWORD]@db.[PROJECT].supabase.co:5432/postgres?sslmode=require"

go run cmd/migrate/main.go up
```

### 2. Deploy Application

```yaml
# docker-compose.production.yml
version: '3.8'
services:
  archivus:
    image: archivus:latest
    environment:
      - ENVIRONMENT=production
      - DATABASE_URL=${SUPABASE_DATABASE_URL}
      - SUPABASE_URL=${SUPABASE_URL}
      - SUPABASE_ANON_KEY=${SUPABASE_ANON_KEY}
      - STORAGE_TYPE=supabase
      - ENABLE_AI_PROCESSING=true
    ports:
      - "8080:8080"
```

### 3. Supabase Edge Functions (Optional)

```typescript
// supabase/functions/document-webhook/index.ts
// Handle document processing webhooks
import { serve } from "https://deno.land/std@0.168.0/http/server.ts"

serve(async (req) => {
  const { document_id, processing_type } = await req.json()
  
  // Trigger AI processing in our Go backend
  const response = await fetch('https://your-archivus-api.com/api/v1/ai/process', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ document_id, processing_type })
  })
  
  return new Response(JSON.stringify({ success: true }))
})
```

## 🎯 Migration Strategy

### Phase 1: Database Migration (Immediate)
1. ✅ Create Supabase project
2. ✅ Enable extensions (`uuid-ossp`, `vector`)
3. ✅ Update `DATABASE_URL` to Supabase
4. ✅ Run existing migrations
5. ✅ Test all endpoints

### Phase 2: Storage Migration (Week 1)
1. ✅ Implement Supabase Storage service
2. ✅ Migrate existing files
3. ✅ Update file upload/download endpoints

### Phase 3: Enhanced Features (Week 2-3)
1. ✅ Implement RLS policies
2. ✅ Add real-time subscriptions
3. ✅ Optional: Integrate Supabase Auth

## 🔥 Supabase Advantages for Archivus

### 🚀 **Performance**
- **Global CDN** for document delivery
- **Connection pooling** built-in
- **Auto-scaling** database

### 💰 **Cost Effective**
- **$25/month** for production-ready setup
- **Included**: 500MB database, 1GB storage, 100GB bandwidth
- **Much cheaper** than AWS RDS + S3 + CloudFront

### 🛠️ **Developer Experience**
- **Dashboard UI** for database management
- **Built-in API documentation**
- **Real-time logs** and monitoring
- **One-click backups**

### 🔒 **Security**
- **SSL by default**
- **Row Level Security** for multi-tenancy
- **Built-in authentication**
- **GDPR compliant**

## 📊 Before/After Comparison

### Current Setup (Self-hosted)
```bash
Costs: $50-100/month (VPS + Database + Storage)
Maintenance: High (security, backups, scaling)
Features: Custom implementation
```

### With Supabase
```bash
Costs: $25/month (all included)
Maintenance: Minimal (managed service)
Features: Enhanced (real-time, auth, storage)
```

## 🎉 Conclusion

**Archivus + Supabase = Perfect Match!** 

Your current architecture requires **zero changes** to work with Supabase. You'll get:
- ✅ **Reduced infrastructure costs**
- ✅ **Enhanced security and performance**
- ✅ **Real-time collaboration features**
- ✅ **Global CDN for document delivery**
- ✅ **Managed backups and scaling**

The migration is as simple as **changing the DATABASE_URL** - everything else works out of the box! 🚀 