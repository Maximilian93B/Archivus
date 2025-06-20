-- Test RLS Helper Functions
-- Run this in Supabase SQL Editor to test if the functions work

-- Test the helper functions
SELECT 
  'Testing RLS Helper Functions' as test_name,
  public.get_tenant_id() as extracted_tenant_id,
  public.get_user_role() as extracted_user_role,
  public.is_user_admin() as is_admin,
  auth.jwt() as full_jwt_claims;

-- Test if we can query documents with RLS enabled
SELECT 
  'Testing Documents Query' as test_name,
  COUNT(*) as document_count
FROM documents;

-- Check if RLS is actually enabled on tables
SELECT 
  schemaname, 
  tablename, 
  rowsecurity as rls_enabled
FROM pg_tables 
WHERE schemaname = 'public' 
AND tablename IN ('documents', 'users', 'tenants')
ORDER BY tablename; 