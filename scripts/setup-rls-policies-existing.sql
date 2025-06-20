-- Archivus RLS (Row Level Security) Setup - Existing Tables Only
-- This script enables RLS and creates policies for multi-tenant data isolation
-- Run this in your Supabase SQL Editor

-- Enable RLS on existing Archivus tables only
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE folders ENABLE ROW LEVEL SECURITY;
ALTER TABLE tags ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_tags ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE shares ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_comments ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_analytics ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflows ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow_tasks ENABLE ROW LEVEL SECURITY;

-- Helper function to extract tenant_id from JWT
CREATE OR REPLACE FUNCTION auth.tenant_id() RETURNS uuid AS $$
  SELECT COALESCE(
    auth.jwt() ->> 'tenant_id',
    (auth.jwt() -> 'user_metadata' ->> 'tenant_id')
  )::uuid;
$$ LANGUAGE sql STABLE;

-- Helper function to extract user role from JWT
CREATE OR REPLACE FUNCTION auth.user_role() RETURNS text AS $$
  SELECT COALESCE(
    auth.jwt() ->> 'role',
    (auth.jwt() -> 'user_metadata' ->> 'role'),
    'user'
  );
$$ LANGUAGE sql STABLE;

-- Helper function to check if user is admin
CREATE OR REPLACE FUNCTION auth.is_admin() RETURNS boolean AS $$
  SELECT auth.user_role() = 'admin';
$$ LANGUAGE sql STABLE;

-- TENANT POLICIES
CREATE POLICY "Users can only access their own tenant" ON tenants
  FOR ALL USING (id = auth.tenant_id());

CREATE POLICY "Admins can manage their tenant" ON tenants
  FOR ALL USING (id = auth.tenant_id() AND auth.is_admin());

-- USER POLICIES  
CREATE POLICY "Users can only access users in their tenant" ON users
  FOR SELECT USING (tenant_id = auth.tenant_id());

CREATE POLICY "Users can update their own profile" ON users
  FOR UPDATE USING (tenant_id = auth.tenant_id() AND id = auth.uid()::uuid);

CREATE POLICY "Admins can manage users in their tenant" ON users
  FOR ALL USING (tenant_id = auth.tenant_id() AND auth.is_admin());

-- DOCUMENT POLICIES
CREATE POLICY "Users can access documents in their tenant" ON documents
  FOR SELECT USING (tenant_id = auth.tenant_id());

CREATE POLICY "Users can create documents in their tenant" ON documents
  FOR INSERT WITH CHECK (tenant_id = auth.tenant_id());

CREATE POLICY "Users can update their own documents or admins can update any" ON documents
  FOR UPDATE USING (
    tenant_id = auth.tenant_id() AND 
    (created_by = auth.uid()::uuid OR auth.is_admin())
  );

CREATE POLICY "Users can delete their own documents or admins can delete any" ON documents
  FOR DELETE USING (
    tenant_id = auth.tenant_id() AND 
    (created_by = auth.uid()::uuid OR auth.is_admin())
  );

-- DOCUMENT VERSION POLICIES
CREATE POLICY "Users can access document versions in their tenant" ON document_versions
  FOR SELECT USING (
    EXISTS (
      SELECT 1 FROM documents d 
      WHERE d.id = document_versions.document_id 
      AND d.tenant_id = auth.tenant_id()
    )
  );

CREATE POLICY "Users can create document versions in their tenant" ON document_versions
  FOR INSERT WITH CHECK (
    EXISTS (
      SELECT 1 FROM documents d 
      WHERE d.id = document_versions.document_id 
      AND d.tenant_id = auth.tenant_id()
    )
  );

-- FOLDER POLICIES
CREATE POLICY "Users can access folders in their tenant" ON folders
  FOR SELECT USING (tenant_id = auth.tenant_id());

CREATE POLICY "Users can create folders in their tenant" ON folders
  FOR INSERT WITH CHECK (tenant_id = auth.tenant_id());

CREATE POLICY "Users can update folders in their tenant" ON folders
  FOR UPDATE USING (tenant_id = auth.tenant_id());

CREATE POLICY "Users can delete folders in their tenant" ON folders
  FOR DELETE USING (tenant_id = auth.tenant_id());

-- TAG POLICIES
CREATE POLICY "Users can access tags in their tenant" ON tags
  FOR SELECT USING (tenant_id = auth.tenant_id());

CREATE POLICY "Users can create tags in their tenant" ON tags
  FOR INSERT WITH CHECK (tenant_id = auth.tenant_id());

CREATE POLICY "Users can update tags in their tenant" ON tags
  FOR UPDATE USING (tenant_id = auth.tenant_id());

CREATE POLICY "Users can delete tags in their tenant" ON tags
  FOR DELETE USING (tenant_id = auth.tenant_id());

-- CATEGORY POLICIES
CREATE POLICY "Users can access categories in their tenant" ON categories
  FOR SELECT USING (tenant_id = auth.tenant_id());

CREATE POLICY "Users can create categories in their tenant" ON categories
  FOR INSERT WITH CHECK (tenant_id = auth.tenant_id());

CREATE POLICY "Users can update categories in their tenant" ON categories
  FOR UPDATE USING (tenant_id = auth.tenant_id());

CREATE POLICY "Users can delete categories in their tenant" ON categories
  FOR DELETE USING (tenant_id = auth.tenant_id());

-- DOCUMENT-CATEGORY JUNCTION POLICIES
CREATE POLICY "Users can access document categories in their tenant" ON document_categories
  FOR SELECT USING (
    EXISTS (
      SELECT 1 FROM documents d 
      WHERE d.id = document_categories.document_id 
      AND d.tenant_id = auth.tenant_id()
    )
  );

CREATE POLICY "Users can manage document categories in their tenant" ON document_categories
  FOR ALL USING (
    EXISTS (
      SELECT 1 FROM documents d 
      WHERE d.id = document_categories.document_id 
      AND d.tenant_id = auth.tenant_id()
    )
  );

-- DOCUMENT-TAG JUNCTION POLICIES
CREATE POLICY "Users can access document tags in their tenant" ON document_tags
  FOR SELECT USING (
    EXISTS (
      SELECT 1 FROM documents d 
      WHERE d.id = document_tags.document_id 
      AND d.tenant_id = auth.tenant_id()
    )
  );

CREATE POLICY "Users can manage document tags in their tenant" ON document_tags
  FOR ALL USING (
    EXISTS (
      SELECT 1 FROM documents d 
      WHERE d.id = document_tags.document_id 
      AND d.tenant_id = auth.tenant_id()
    )
  );

-- AUDIT LOG POLICIES
CREATE POLICY "Users can read audit logs in their tenant" ON audit_logs
  FOR SELECT USING (tenant_id = auth.tenant_id());

CREATE POLICY "System can create audit logs" ON audit_logs
  FOR INSERT WITH CHECK (tenant_id = auth.tenant_id());

-- SHARE POLICIES
CREATE POLICY "Users can access shares in their tenant" ON shares
  FOR SELECT USING (tenant_id = auth.tenant_id());

CREATE POLICY "Users can create shares in their tenant" ON shares
  FOR INSERT WITH CHECK (tenant_id = auth.tenant_id());

CREATE POLICY "Users can update their own shares" ON shares
  FOR UPDATE USING (
    tenant_id = auth.tenant_id() AND 
    created_by = auth.uid()::uuid
  );

CREATE POLICY "Users can delete their own shares" ON shares
  FOR DELETE USING (
    tenant_id = auth.tenant_id() AND 
    created_by = auth.uid()::uuid
  );

-- NOTIFICATION POLICIES
CREATE POLICY "Users can access their own notifications" ON notifications
  FOR SELECT USING (
    tenant_id = auth.tenant_id() AND 
    user_id = auth.uid()::uuid
  );

CREATE POLICY "System can create notifications" ON notifications
  FOR INSERT WITH CHECK (tenant_id = auth.tenant_id());

CREATE POLICY "Users can update their own notifications" ON notifications
  FOR UPDATE USING (
    tenant_id = auth.tenant_id() AND 
    user_id = auth.uid()::uuid
  );

-- DOCUMENT TEMPLATE POLICIES
CREATE POLICY "Users can access templates in their tenant" ON document_templates
  FOR SELECT USING (tenant_id = auth.tenant_id());

CREATE POLICY "Admins can manage templates in their tenant" ON document_templates
  FOR ALL USING (tenant_id = auth.tenant_id() AND auth.is_admin());

-- DOCUMENT COMMENT POLICIES
CREATE POLICY "Users can access comments on documents in their tenant" ON document_comments
  FOR SELECT USING (
    EXISTS (
      SELECT 1 FROM documents d 
      WHERE d.id = document_comments.document_id 
      AND d.tenant_id = auth.tenant_id()
    )
  );

CREATE POLICY "Users can create comments on documents in their tenant" ON document_comments
  FOR INSERT WITH CHECK (
    EXISTS (
      SELECT 1 FROM documents d 
      WHERE d.id = document_comments.document_id 
      AND d.tenant_id = auth.tenant_id()
    )
  );

CREATE POLICY "Users can update their own comments" ON document_comments
  FOR UPDATE USING (
    user_id = auth.uid()::uuid AND
    EXISTS (
      SELECT 1 FROM documents d 
      WHERE d.id = document_comments.document_id 
      AND d.tenant_id = auth.tenant_id()
    )
  );

-- DOCUMENT ANALYTICS POLICIES
CREATE POLICY "Users can access analytics in their tenant" ON document_analytics
  FOR SELECT USING (tenant_id = auth.tenant_id());

CREATE POLICY "System can create analytics" ON document_analytics
  FOR INSERT WITH CHECK (tenant_id = auth.tenant_id());

-- WORKFLOW POLICIES
CREATE POLICY "Users can access workflows in their tenant" ON workflows
  FOR SELECT USING (tenant_id = auth.tenant_id());

CREATE POLICY "Admins can manage workflows in their tenant" ON workflows
  FOR ALL USING (tenant_id = auth.tenant_id() AND auth.is_admin());

-- WORKFLOW TASK POLICIES
CREATE POLICY "Users can access workflow tasks in their tenant" ON workflow_tasks
  FOR SELECT USING (
    EXISTS (
      SELECT 1 FROM workflows w 
      WHERE w.id = workflow_tasks.workflow_id 
      AND w.tenant_id = auth.tenant_id()
    )
  );

CREATE POLICY "Users can update assigned workflow tasks" ON workflow_tasks
  FOR UPDATE USING (
    assigned_to = auth.uid()::uuid AND
    EXISTS (
      SELECT 1 FROM workflows w 
      WHERE w.id = workflow_tasks.workflow_id 
      AND w.tenant_id = auth.tenant_id()
    )
  );

-- Grant necessary permissions to authenticated users
GRANT USAGE ON SCHEMA public TO authenticated;
GRANT ALL ON ALL TABLES IN SCHEMA public TO authenticated;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO authenticated;

-- Grant permissions to service role (for admin operations)
GRANT ALL ON ALL TABLES IN SCHEMA public TO service_role;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO service_role;

-- Refresh the schema cache
NOTIFY pgrst, 'reload schema';

-- Verification queries
SELECT schemaname, tablename, rowsecurity 
FROM pg_tables 
WHERE schemaname = 'public' 
AND tablename IN (
  'tenants', 'users', 'documents', 'folders', 'tags', 'categories',
  'document_categories', 'document_tags', 'audit_logs', 'shares', 
  'notifications', 'document_templates', 'document_comments', 
  'document_analytics', 'document_versions', 'workflows', 'workflow_tasks'
)
ORDER BY tablename; 