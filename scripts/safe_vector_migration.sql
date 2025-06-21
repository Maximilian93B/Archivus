-- SAFE Vector Extension Migration (BACKUP FIRST!)
-- This approach preserves your existing vector data

-- Step 1: Check current vector usage
SELECT 
    schemaname, 
    tablename, 
    column_name, 
    data_type
FROM information_schema.columns 
WHERE data_type LIKE '%vector%' 
    OR udt_name LIKE '%vector%';

-- Step 2: Backup vector data (if any exists)
-- CREATE TABLE vector_backup AS 
-- SELECT id, embedding FROM documents WHERE embedding IS NOT NULL;

-- Step 3: Check if extensions schema exists
CREATE SCHEMA IF NOT EXISTS extensions;

-- Step 4: Check current extension location
SELECT 
    extname,
    nspname as schema_name
FROM pg_extension e
JOIN pg_namespace n ON e.extnamespace = n.oid
WHERE extname = 'vector';

-- WARNING: The following commands are DESTRUCTIVE
-- Only proceed if you have a full database backup!

-- Step 5: Drop dependent objects first (DANGEROUS!)
-- ALTER TABLE documents DROP COLUMN IF EXISTS embedding;

-- Step 6: Drop and recreate extension
-- DROP EXTENSION IF EXISTS vector;
-- CREATE EXTENSION vector WITH SCHEMA extensions;

-- Step 7: Recreate dependent columns
-- ALTER TABLE documents ADD COLUMN embedding extensions.vector(1536);

-- Step 8: Restore data (if backed up)
-- UPDATE documents SET embedding = vector_backup.embedding 
-- FROM vector_backup WHERE documents.id = vector_backup.id;

-- RECOMMENDATION: DON'T RUN THE DESTRUCTIVE COMMANDS
-- The warning is not critical for your application security 