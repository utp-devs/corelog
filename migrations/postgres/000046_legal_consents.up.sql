-- Versioned legal documents and the auditable record of the consent each user gave to them.
CREATE TABLE IF NOT EXISTS legal_documents (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug           TEXT        NOT NULL CHECK (slug IN ('privacy_policy', 'data_processing', 'terms_of_use', 'cookies', 'communications')),
    version        INT         NOT NULL CHECK (version >= 1),
    title          TEXT        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    body_markdown  TEXT        NOT NULL CHECK (char_length(body_markdown) >= 1),
    summary        TEXT        NOT NULL DEFAULT '',
    required       BOOLEAN     NOT NULL DEFAULT TRUE,
    effective_from TIMESTAMPTZ NOT NULL,
    published_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by     UUID        NULL REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT legal_documents_slug_version_key UNIQUE (slug, version),
    -- The optional communications consent can never be mandatory.
    CONSTRAINT legal_documents_communications_optional CHECK (slug <> 'communications' OR NOT required)
);

CREATE INDEX IF NOT EXISTS legal_documents_created_by_idx ON legal_documents (created_by) WHERE created_by IS NOT NULL;

CREATE TABLE IF NOT EXISTS user_consents (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    document_slug    TEXT        NOT NULL CHECK (document_slug IN ('privacy_policy', 'data_processing', 'terms_of_use', 'cookies', 'communications')),
    document_version INT         NOT NULL CHECK (document_version >= 1),
    accepted_at      TIMESTAMPTZ NOT NULL,
    -- Anonymized before storing: IPv4 with the last octet zeroed, IPv6 truncated to its /64 prefix.
    ip               INET        NULL,
    user_agent       TEXT        NULL CHECK (user_agent IS NULL OR char_length(user_agent) <= 255),
    source           TEXT        NOT NULL CHECK (source IN ('register', 'invite', 'login_reprompt', 'profile')),
    revoked_at       TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS user_consents_user_doc_idx ON user_consents (user_id, document_slug, document_version);
-- Accepting the same version twice is a no-op while the first acceptance is still active.
CREATE UNIQUE INDEX IF NOT EXISTS user_consents_active_uniq
    ON user_consents (user_id, document_slug, document_version) WHERE revoked_at IS NULL;
