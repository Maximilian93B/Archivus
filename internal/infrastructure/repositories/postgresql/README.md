# PostgreSQL Repository Implementation

This package contains the PostgreSQL implementations of all repository interfaces defined in the domain layer.

## Implemented Repositories

### ✅ Core Repositories
- **TenantRepository** - Multi-tenant functionality
- **UserRepository** - User management and authentication
- **DocumentRepository** - Document storage and retrieval

### 🚧 TODO Repositories
- **FolderRepository** - Folder hierarchy management
- **TagRepository** - Document tagging system
- **CategoryRepository** - Document categorization
- **WorkflowRepository** - Business process workflows
- **WorkflowTaskRepository** - Workflow task management
- **AIProcessingJobRepository** - AI processing queue
- **AuditLogRepository** - Audit trail logging
- **ShareRepository** - Document sharing functionality
- **AnalyticsRepository** - Business intelligence queries
- **NotificationRepository** - User notifications

## Features

### Multi-tenancy
All repositories properly handle tenant isolation:
- Documents are isolated by `tenant_id`
- Users are scoped to their tenant
- Cross-tenant data access is prevented

### Error Handling
- Proper error wrapping with context
- Domain-specific error types
- Database constraint violations handled gracefully

### Performance
- Database indexes for common queries
- Preloading of related entities
- Efficient pagination implementation

### Testing
Each repository includes comprehensive tests:
- Unit tests for all CRUD operations
- Error condition testing
- Multi-tenant isolation verification
- Test utilities for easy setup

## Usage

```go
// Initialize database connection
db, err := database.New(databaseURL)
if err != nil {
    return err
}

// Create repositories
repos := postgresql.NewRepositories(db)

// Use repositories
tenant, err := repos.TenantRepo.GetBySubdomain(ctx, "acme")
if err != nil {
    return err
}

users, total, err := repos.UserRepo.ListByTenant(ctx, tenant.ID, params)
if err != nil {
    return err
}
```

## Running Tests

```bash
# Run all repository tests
make test-repos

# Run specific repository tests
go test -v ./internal/infrastructure/repositories/postgresql/

# Run with coverage
go test -v -coverprofile=coverage.out ./internal/infrastructure/repositories/postgresql/
```

## Database Schema

The repositories work with the following key entities:
- `tenants` - Multi-tenant organizations
- `users` - User accounts with role-based access
- `documents` - File storage with metadata
- `folders` - Hierarchical organization
- `tags` - Document tagging
- `categories` - Document categorization
- `workflows` - Business process automation
- `ai_processing_jobs` - AI processing queue
- `audit_logs` - Complete audit trail 