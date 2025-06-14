-- Initialize PostgreSQL database for Archivus
-- This script runs automatically when the database container starts

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- Create test database for running tests
CREATE DATABASE archivus_test;

-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE archivus_dev TO postgres;
GRANT ALL PRIVILEGES ON DATABASE archivus_test TO postgres;

-- Optional: Create a read-only user for analytics
-- CREATE USER archivus_readonly WITH PASSWORD 'readonly_password';
-- GRANT CONNECT ON DATABASE archivus_dev TO archivus_readonly;
-- GRANT USAGE ON SCHEMA public TO archivus_readonly;
-- GRANT SELECT ON ALL TABLES IN SCHEMA public TO archivus_readonly;
-- ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO archivus_readonly; 