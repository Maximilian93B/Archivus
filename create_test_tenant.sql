-- Create test tenant for Archivus pipeline testing
-- This should be run against the PostgreSQL database

-- Insert test tenant
INSERT INTO tenants (
    id, 
    name, 
    subdomain, 
    subscription_tier, 
    storage_quota,
    api_quota,
    settings,
    is_active, 
    created_at, 
    updated_at
) VALUES (
    gen_random_uuid(),
    'Test Corporation',
    'testcorp',
    'starter',
    5368709120,  -- 5GB default
    1000,        -- API quota
    '{}',        -- Empty JSON settings
    true,
    now(),
    now()
) ON CONFLICT (subdomain) DO NOTHING;

-- Verify the tenant was created
SELECT id, name, subdomain, subscription_tier, is_active, created_at 
FROM tenants 
WHERE subdomain = 'testcorp'; 