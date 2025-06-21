-- Archivus - Comprehensive Supabase Security Fix
-- Run this script in your Supabase SQL Editor to resolve all security warnings

-- =======================================================
-- 1. ENABLE RLS ON MISSING TABLES
-- =======================================================

-- Enable RLS on ai_processing_jobs table if it exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'ai_processing_jobs' AND table_schema = 'public') THEN
        -- Enable RLS
        ALTER TABLE public.ai_processing_jobs ENABLE ROW LEVEL SECURITY;
        
        -- Drop existing policies to avoid conflicts
        DROP POLICY IF EXISTS "Users can access their tenant's AI processing jobs" ON public.ai_processing_jobs;
        DROP POLICY IF EXISTS "Users can access AI jobs in their tenant" ON public.ai_processing_jobs;
        DROP POLICY IF EXISTS "Users can create AI jobs in their tenant" ON public.ai_processing_jobs;
        DROP POLICY IF EXISTS "System can update AI jobs" ON public.ai_processing_jobs;
        
        -- Create comprehensive RLS policies
        CREATE POLICY "Users can access AI jobs in their tenant" ON public.ai_processing_jobs
            FOR SELECT USING (tenant_id = public.get_tenant_id());
            
        CREATE POLICY "Users can create AI jobs in their tenant" ON public.ai_processing_jobs
            FOR INSERT WITH CHECK (tenant_id = public.get_tenant_id());
            
        CREATE POLICY "System can update AI jobs" ON public.ai_processing_jobs
            FOR UPDATE USING (tenant_id = public.get_tenant_id());
            
        CREATE POLICY "System can delete AI jobs" ON public.ai_processing_jobs
            FOR DELETE USING (tenant_id = public.get_tenant_id());
            
        RAISE NOTICE 'RLS enabled on ai_processing_jobs table with full policies';
    ELSE
        RAISE NOTICE 'ai_processing_jobs table does not exist, skipping RLS setup';
    END IF;
END
$$;

-- =======================================================
-- 2. SECURE RLS HELPER FUNCTIONS (Fix Search Path Issues)
-- =======================================================

-- Create secure utility schema for helper functions
CREATE SCHEMA IF NOT EXISTS utils;

-- Create secure helper function to get tenant ID from JWT
CREATE OR REPLACE FUNCTION public.get_tenant_id()
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
STABLE
SET search_path = public, auth
AS $$
DECLARE
    tenant_id_text text;
    tenant_uuid uuid;
BEGIN
    -- Extract tenant_id from JWT user_metadata
    SELECT COALESCE(
        auth.jwt() ->> 'tenant_id',
        (auth.jwt() -> 'user_metadata' ->> 'tenant_id')
    ) INTO tenant_id_text;
    
    -- Return NULL if no tenant_id found
    IF tenant_id_text IS NULL OR tenant_id_text = '' THEN
        RETURN NULL;
    END IF;
    
    -- Convert to UUID, return NULL if invalid
    BEGIN
        tenant_uuid := tenant_id_text::uuid;
        RETURN tenant_uuid;
    EXCEPTION WHEN invalid_text_representation THEN
        RETURN NULL;
    END;
END;
$$;

-- Create secure helper function to get user role from JWT
CREATE OR REPLACE FUNCTION public.get_user_role()
RETURNS text
LANGUAGE plpgsql
SECURITY DEFINER
STABLE
SET search_path = public, auth
AS $$
DECLARE
    user_role text;
BEGIN
    -- Extract role from JWT user_metadata
    SELECT COALESCE(
        auth.jwt() ->> 'role',
        (auth.jwt() -> 'user_metadata' ->> 'role'),
        'user'  -- default role
    ) INTO user_role;
    
    RETURN user_role;
END;
$$;

-- Create secure helper function to check if user is admin
CREATE OR REPLACE FUNCTION public.is_user_admin()
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
STABLE
SET search_path = public
AS $$
BEGIN
    RETURN public.get_user_role() = 'admin';
END;
$$;

-- =======================================================
-- 3. MOVE VECTOR EXTENSION TO PROPER SCHEMA
-- =======================================================

-- Note: This cannot be automated - must be done manually in Supabase Dashboard
-- Go to Database > Extensions > Find 'vector' extension > Move to 'extensions' schema

-- =======================================================
-- 4. GRANT PROPER PERMISSIONS
-- =======================================================

-- Grant execute permissions to authenticated users
GRANT EXECUTE ON FUNCTION public.get_tenant_id() TO authenticated;
GRANT EXECUTE ON FUNCTION public.get_user_role() TO authenticated;
GRANT EXECUTE ON FUNCTION public.is_user_admin() TO authenticated;

-- Grant execute permissions to anon users (for login)
GRANT EXECUTE ON FUNCTION public.get_tenant_id() TO anon;
GRANT EXECUTE ON FUNCTION public.get_user_role() TO anon;
GRANT EXECUTE ON FUNCTION public.is_user_admin() TO anon;

-- Grant usage on utils schema
GRANT USAGE ON SCHEMA utils TO authenticated, anon;

-- =======================================================
-- 5. VERIFICATION QUERIES
-- =======================================================

-- Check RLS status on all tables
SELECT 
    schemaname, 
    tablename, 
    rowsecurity as rls_enabled,
    CASE 
        WHEN rowsecurity THEN '✅ RLS Enabled'
        ELSE '⚠️ RLS DISABLED'
    END as status
FROM pg_tables 
WHERE schemaname = 'public' 
    AND tablename IN (
        'tenants', 'users', 'documents', 'folders', 'tags', 'categories',
        'document_categories', 'document_tags', 'ai_processing_jobs', 
        'audit_logs', 'shares', 'notifications', 'document_templates',
        'document_comments', 'document_analytics', 'document_versions'
    )
ORDER BY tablename;

-- Check for any remaining tables without RLS
SELECT 
    'WARNING: Tables without RLS' as alert,
    schemaname,
    tablename
FROM pg_tables 
WHERE schemaname = 'public' 
    AND tablename NOT IN ('spatial_ref_sys', 'geography_columns', 'geometry_columns')
    AND rowsecurity = false
ORDER BY tablename;

-- Verify helper functions exist and are secure
SELECT 
    'Helper Functions Status' as category,
    proname as function_name,
    prosecdef as is_security_definer,
    CASE 
        WHEN prosecdef THEN '✅ Secure'
        ELSE '⚠️ Not Secure'
    END as security_status
FROM pg_proc 
WHERE proname IN ('get_tenant_id', 'get_user_role', 'is_user_admin')
    AND pronamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public');

-- Test the helper functions (should work without errors)
SELECT 
    'Function Test' as test_category,
    public.get_tenant_id() as tenant_id_result,
    public.get_user_role() as user_role_result,
    public.is_user_admin() as is_admin_result;

SELECT '🎉 Supabase Security Fix Applied Successfully!' as completion_status; 