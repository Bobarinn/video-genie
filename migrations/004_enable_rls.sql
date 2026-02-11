-- Migration 004: Enable Row Level Security on all tables
--
-- Even though the Go backend connects via service_role / secret key (which has
-- the BYPASSRLS attribute and is unaffected), enabling RLS is a defense-in-depth
-- measure that ensures:
--
--   1. The anon / publishable key has ZERO access to any table by default.
--   2. If a frontend is ever introduced with direct Supabase access, data is
--      protected out of the box — you only grant what you explicitly allow.
--   3. Supabase Security Advisor stops flagging unprotected tables.
--
-- No permissive policies are created here. That means:
--   - anon role          → blocked (no policies = no access)
--   - authenticated role → blocked (no policies = no access)
--   - service_role       → full access (BYPASSRLS, unaffected by RLS)
--
-- When you add a frontend or public API that connects with the anon/publishable
-- key, create targeted SELECT/INSERT/UPDATE/DELETE policies per table as needed.

-- ── Enable RLS ──────────────────────────────────────────────────────────

ALTER TABLE series           ENABLE ROW LEVEL SECURITY;
ALTER TABLE graphics_presets ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects         ENABLE ROW LEVEL SECURITY;
ALTER TABLE clips            ENABLE ROW LEVEL SECURITY;
ALTER TABLE assets           ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobs             ENABLE ROW LEVEL SECURITY;

-- ── Force RLS for table owners too (optional extra safety) ──────────────
-- By default, table owners bypass RLS. FORCE ensures even the table owner
-- is subject to policies, unless they have the BYPASSRLS attribute.
-- This prevents accidental data leaks if a non-service_role connection
-- is ever misconfigured as the table owner.

ALTER TABLE series           FORCE ROW LEVEL SECURITY;
ALTER TABLE graphics_presets FORCE ROW LEVEL SECURITY;
ALTER TABLE projects         FORCE ROW LEVEL SECURITY;
ALTER TABLE clips            FORCE ROW LEVEL SECURITY;
ALTER TABLE assets           FORCE ROW LEVEL SECURITY;
ALTER TABLE jobs             FORCE ROW LEVEL SECURITY;
