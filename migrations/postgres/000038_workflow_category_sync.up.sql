-- Workflow administration: when the category of an organization status changes, the legacy mirror of
-- its issues is refreshed in the same transaction. The status_id of those issues must stay as it is,
-- so the sync trigger honours the transaction-local setting corelog.keep_status_id set by the writer.
-- Every other writer keeps the behaviour of migration 000037.

CREATE OR REPLACE FUNCTION corelog_sync_issue_status() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    v_cat text;
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.status_id IS NULL THEN
            NEW.status_id := corelog_status_for_legacy(NEW.project_id, NEW.status);
        ELSE
            SELECT category INTO v_cat FROM statuses WHERE id = NEW.status_id;
            IF v_cat IS NOT NULL THEN
                NEW.status := corelog_legacy_status(v_cat);
            END IF;
        END IF;
        RETURN NEW;
    END IF;
    IF NEW.status_id IS DISTINCT FROM OLD.status_id THEN
        SELECT category INTO v_cat FROM statuses WHERE id = NEW.status_id;
        IF v_cat IS NOT NULL THEN
            NEW.status := corelog_legacy_status(v_cat);
        END IF;
    ELSIF NEW.project_id IS DISTINCT FROM OLD.project_id THEN
        NEW.status_id := corelog_status_for_legacy(NEW.project_id, NEW.status);
        SELECT category INTO v_cat FROM statuses WHERE id = NEW.status_id;
        IF v_cat IS NOT NULL THEN
            NEW.status := corelog_legacy_status(v_cat);
        END IF;
    ELSIF NEW.status IS DISTINCT FROM OLD.status THEN
        IF coalesce(current_setting('corelog.keep_status_id', true), '') = 'on' THEN
            NEW.status_id := OLD.status_id;
        ELSE
            NEW.status_id := corelog_status_for_legacy(NEW.project_id, NEW.status);
        END IF;
    END IF;
    RETURN NEW;
END
$$;
