-- Fix RLS Helper Functions with Proper Security
-- This script addresses the "Function Search Path Mutable" issues shown in Supabase dashboard
-- Using CREATE OR REPLACE to avoid dependency issues with existing policies

-- Create secure helper function to get tenant ID from JWT
CREATE OR REPLACE FUNCTION public.get_tenant_id()
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
STABLE
SET search_path = public
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
SET search_path = public
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

-- Grant execute permissions to authenticated users
GRANT EXECUTE ON FUNCTION public.get_tenant_id() TO authenticated;
GRANT EXECUTE ON FUNCTION public.get_user_role() TO authenticated;
GRANT EXECUTE ON FUNCTION public.is_user_admin() TO authenticated;

-- Grant execute permissions to anon users (for login)
GRANT EXECUTE ON FUNCTION public.get_tenant_id() TO anon;
GRANT EXECUTE ON FUNCTION public.get_user_role() TO anon;
GRANT EXECUTE ON FUNCTION public.is_user_admin() TO anon;

-- Test the functions
SELECT 'RLS helper functions updated successfully' as status; 