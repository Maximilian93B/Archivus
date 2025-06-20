-- Enable RLS on missing tables
-- This addresses the "RLS Disabled in Public" issue for ai_processing_jobs

-- Enable RLS on ai_processing_jobs table if it exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'ai_processing_jobs' AND table_schema = 'public') THEN
        -- Enable RLS
        ALTER TABLE public.ai_processing_jobs ENABLE ROW LEVEL SECURITY;
        
        -- Create RLS policy for ai_processing_jobs
        DROP POLICY IF EXISTS "Users can access their tenant's AI processing jobs" ON public.ai_processing_jobs;
        CREATE POLICY "Users can access their tenant's AI processing jobs" ON public.ai_processing_jobs
            FOR ALL USING (tenant_id = public.get_tenant_id());
            
        RAISE NOTICE 'RLS enabled on ai_processing_jobs table';
    ELSE
        RAISE NOTICE 'ai_processing_jobs table does not exist, skipping RLS setup';
    END IF;
END
$$;

-- Check for any other tables missing RLS
SELECT 
    schemaname,
    tablename,
    rowsecurity as rls_enabled
FROM pg_tables 
WHERE schemaname = 'public' 
    AND tablename NOT IN ('spatial_ref_sys', 'geography_columns', 'geometry_columns')
    AND rowsecurity = false
ORDER BY tablename; 