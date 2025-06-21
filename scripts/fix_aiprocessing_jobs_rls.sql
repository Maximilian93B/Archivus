-- Fix RLS for a_iprocessing_jobs table (correct table name)
-- The previous script was looking for 'ai_processing_jobs' but the actual table is 'a_iprocessing_jobs'

-- First, let's check if the table exists and its current RLS status
SELECT 
    schemaname, 
    tablename, 
    rowsecurity as rls_enabled,
    CASE 
        WHEN rowsecurity THEN '✅ RLS Enabled'
        ELSE '❌ RLS DISABLED'
    END as status
FROM pg_tables 
WHERE schemaname = 'public' 
    AND tablename = 'a_iprocessing_jobs';

-- Enable RLS on the correct table name
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'a_iprocessing_jobs' AND table_schema = 'public') THEN
        -- Enable RLS
        ALTER TABLE public.a_iprocessing_jobs ENABLE ROW LEVEL SECURITY;
        
        -- Drop existing policies to avoid conflicts
        DROP POLICY IF EXISTS "Users can access their tenant's AI processing jobs" ON public.a_iprocessing_jobs;
        DROP POLICY IF EXISTS "Users can access AI jobs in their tenant" ON public.a_iprocessing_jobs;
        DROP POLICY IF EXISTS "Users can create AI jobs in their tenant" ON public.a_iprocessing_jobs;
        DROP POLICY IF EXISTS "System can update AI jobs" ON public.a_iprocessing_jobs;
        DROP POLICY IF EXISTS "System can delete AI jobs" ON public.a_iprocessing_jobs;
        
        -- Create comprehensive RLS policies for a_iprocessing_jobs
        CREATE POLICY "Users can access AI jobs in their tenant" ON public.a_iprocessing_jobs
            FOR SELECT USING (tenant_id = public.get_tenant_id());
            
        CREATE POLICY "Users can create AI jobs in their tenant" ON public.a_iprocessing_jobs
            FOR INSERT WITH CHECK (tenant_id = public.get_tenant_id());
            
        CREATE POLICY "System can update AI jobs" ON public.a_iprocessing_jobs
            FOR UPDATE USING (tenant_id = public.get_tenant_id());
            
        CREATE POLICY "System can delete AI jobs" ON public.a_iprocessing_jobs
            FOR DELETE USING (tenant_id = public.get_tenant_id());
            
        RAISE NOTICE '✅ RLS enabled on a_iprocessing_jobs table with full policies';
    ELSE
        RAISE NOTICE '❌ a_iprocessing_jobs table does not exist';
    END IF;
END
$$;

-- Verify the fix worked
SELECT 
    'After Fix - RLS Status' as check_type,
    schemaname, 
    tablename, 
    rowsecurity as rls_enabled,
    CASE 
        WHEN rowsecurity THEN '✅ RLS NOW ENABLED'
        ELSE '❌ RLS STILL DISABLED'
    END as status
FROM pg_tables 
WHERE schemaname = 'public' 
    AND tablename = 'a_iprocessing_jobs';

-- Check if policies were created
SELECT 
    'RLS Policies Created' as check_type,
    schemaname,
    tablename,
    policyname,
    permissive,
    roles,
    cmd,
    qual
FROM pg_policies 
WHERE schemaname = 'public' 
    AND tablename = 'a_iprocessing_jobs';

SELECT '🎉 a_iprocessing_jobs RLS Fix Complete!' as completion_status; 