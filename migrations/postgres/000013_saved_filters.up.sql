CREATE TABLE IF NOT EXISTS saved_filters (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name VARCHAR(80) NOT NULL,
    filters JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT saved_filters_owner_name_unique UNIQUE (owner_id, name),
    CONSTRAINT saved_filters_name_not_blank CHECK (length(trim(name)) > 0),
    CONSTRAINT saved_filters_payload_object CHECK (jsonb_typeof(filters) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_saved_filters_owner_updated ON saved_filters (owner_id, updated_at DESC, id);
