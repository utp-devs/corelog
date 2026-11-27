-- Search: full-text and trigram indexes, saved filters with sharing and subscriptions.
--
-- pg_trgm is created in the public schema (extensions are shared by the whole database) and the trigram
-- indexes are only created when the extension is available, so a restricted role keeps working without them.

DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS pg_trgm SCHEMA public;
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'pg_trgm is not available (%): trigram indexes are skipped', SQLERRM;
END
$$;

-- Weighted search document of an issue: summary (A) and description (B), in the Spanish and the simple
-- configurations so that both stemmed and literal terms match.
ALTER TABLE issues ADD COLUMN IF NOT EXISTS search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('spanish'::regconfig, coalesce(summary, '')), 'A') ||
    setweight(to_tsvector('simple'::regconfig, coalesce(summary, '')), 'A') ||
    setweight(to_tsvector('spanish'::regconfig, coalesce(description, '')), 'B') ||
    setweight(to_tsvector('simple'::regconfig, coalesce(description, '')), 'B')
) STORED;

CREATE INDEX IF NOT EXISTS idx_issues_search_vector ON issues USING GIN (search_vector);

-- Comments are searched through their own document.
CREATE INDEX IF NOT EXISTS idx_comments_fts ON comments USING GIN (to_tsvector('spanish'::regconfig, body))
    WHERE deleted_at IS NULL;

DO $$
DECLARE
    ext_schema text;
BEGIN
    SELECT n.nspname INTO ext_schema
    FROM pg_extension e JOIN pg_namespace n ON n.oid = e.extnamespace
    WHERE e.extname = 'pg_trgm';
    IF ext_schema IS NOT NULL THEN
        EXECUTE format('CREATE INDEX IF NOT EXISTS idx_issues_summary_trgm ON issues USING GIN (summary %I.gin_trgm_ops)', ext_schema);
        EXECUTE format('CREATE INDEX IF NOT EXISTS idx_projects_name_trgm ON projects USING GIN (name %I.gin_trgm_ops)', ext_schema);
        EXECUTE format('CREATE INDEX IF NOT EXISTS idx_projects_key_trgm ON projects USING GIN (key %I.gin_trgm_ops)', ext_schema);
    END IF;
END
$$;

-- Saved filters. query_text is a CoreQL query; columns is the ordered list of columns to show; sort is an
-- ORDER BY list without the keywords (for example "updated DESC").
CREATE TABLE IF NOT EXISTS filters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name            VARCHAR(120) NOT NULL CHECK (char_length(btrim(name)) > 0),
    description     TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    query_text      TEXT NOT NULL DEFAULT '' CHECK (char_length(query_text) <= 4000),
    columns         JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(columns) = 'array'),
    sort            TEXT NOT NULL DEFAULT '' CHECK (char_length(sort) <= 200),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_filters_owner_name ON filters (owner_id, lower(name));
CREATE INDEX IF NOT EXISTS idx_filters_organization ON filters (organization_id);

-- Shares of a filter. target_id points to a user, team, project or organization according to share_type; the
-- column is polymorphic, so it carries no foreign key and the application removes the shares of deleted targets.
CREATE TABLE IF NOT EXISTS filter_shares (
    filter_id  UUID NOT NULL REFERENCES filters (id) ON DELETE CASCADE,
    share_type TEXT NOT NULL CHECK (share_type IN ('user', 'team', 'project', 'organization')),
    target_id  UUID NOT NULL,
    permission TEXT NOT NULL DEFAULT 'view' CHECK (permission IN ('view', 'edit')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (filter_id, share_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_filter_shares_target ON filter_shares (share_type, target_id);

-- Subscriptions: one per user and filter. schedule is {"frequency": "daily"|"weekly", "hour": 0-23,
-- "weekday": 0-6 (Sunday = 0, weekly only), "timezone": IANA name}.
CREATE TABLE IF NOT EXISTS filter_subscriptions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filter_id    UUID NOT NULL REFERENCES filters (id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    schedule     JSONB NOT NULL CHECK (jsonb_typeof(schedule) = 'object'),
    last_sent_at TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (filter_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_filter_subscriptions_user ON filter_subscriptions (user_id);

-- Legacy saved filters become CoreQL filters of the first organization of their owner. The legacy status maps
-- to its category, the free text to a text search and the due range to due-date comparisons.
INSERT INTO filters (id, owner_id, organization_id, name, query_text, created_at, updated_at)
SELECT sf.id, sf.owner_id, org.organization_id, sf.name,
       array_to_string(ARRAY(
           SELECT clause FROM unnest(ARRAY[
               CASE sf.filters ->> 'status'
                   WHEN 'open' THEN 'statusCategory = todo'
                   WHEN 'in_progress' THEN 'statusCategory = in_progress'
                   WHEN 'resolved' THEN 'statusCategory = done'
                   WHEN 'closed' THEN 'statusCategory = done'
               END,
               CASE sf.filters ->> 'priority'
                   WHEN 'critical' THEN 'priority = "Máxima"'
                   WHEN 'high' THEN 'priority = "Alta"'
                   WHEN 'medium' THEN 'priority = "Media"'
                   WHEN 'low' THEN 'priority IN ("Baja", "Mínima")'
               END,
               CASE sf.filters ->> 'issue_type'
                   WHEN 'epic' THEN 'type = "Épica"'
                   WHEN 'story' THEN 'type = "Historia"'
                   WHEN 'task' THEN 'type = "Tarea"'
                   WHEN 'bug' THEN 'type = "Error"'
                   WHEN 'subtask' THEN 'type = "Subtarea"'
               END,
               CASE WHEN coalesce(sf.filters ->> 'query', '') <> ''
                   THEN 'text ~ "' || replace(replace(sf.filters ->> 'query', '\', '\\'), '"', '\"') || '"' END,
               CASE WHEN coalesce(sf.filters ->> 'due_from', '') ~ '^\d{4}-\d{2}-\d{2}'
                   THEN 'due >= "' || substr(sf.filters ->> 'due_from', 1, 10) || '"' END,
               CASE WHEN coalesce(sf.filters ->> 'due_to', '') ~ '^\d{4}-\d{2}-\d{2}'
                   THEN 'due <= "' || substr(sf.filters ->> 'due_to', 1, 10) || '"' END
           ]) AS clause WHERE clause IS NOT NULL), ' AND '),
       sf.created_at, sf.updated_at
FROM saved_filters sf
CROSS JOIN LATERAL (
    SELECT om.organization_id FROM organization_members om
    JOIN organizations o ON o.id = om.organization_id AND o.deleted_at IS NULL
    WHERE om.user_id = sf.owner_id
    ORDER BY om.joined_at, om.organization_id
    LIMIT 1
) org
ON CONFLICT DO NOTHING;
