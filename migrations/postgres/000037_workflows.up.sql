-- Workflows: organization status catalog, one workflow per project (versioned JSON definition),
-- issues.status_id, status history and board column bindings.
--
-- The legacy issues.status column stays and is kept in sync with status_id (the legacy mirror:
-- todo -> open, in_progress -> in_progress, done -> closed) so readers that were not moved yet
-- (board totals, sprints, dashboard, search, calendar) keep working.

-- 1. The legacy per-project transition tables are replaced by the new workflow definition.
DROP TABLE IF EXISTS workflow_transitions;
DROP TABLE IF EXISTS workflows;

CREATE TABLE statuses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name            VARCHAR(60) NOT NULL,
    category        TEXT NOT NULL CHECK (category IN ('todo', 'in_progress', 'done')),
    position        INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT statuses_name_not_blank CHECK (length(btrim(name)) > 0)
);
CREATE UNIQUE INDEX uq_statuses_org_name ON statuses (organization_id, lower(name));

CREATE TABLE workflows (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects (id) ON DELETE CASCADE,
    definition JSONB NOT NULL CHECK (jsonb_typeof(definition) = 'object'),
    version    INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE issues ADD COLUMN status_id UUID NULL REFERENCES statuses (id) ON DELETE RESTRICT;

CREATE TABLE issue_status_history (
    id             BIGSERIAL PRIMARY KEY,
    issue_id       UUID NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    from_status_id UUID NULL REFERENCES statuses (id) ON DELETE SET NULL,
    to_status_id   UUID NOT NULL REFERENCES statuses (id) ON DELETE CASCADE,
    changed_by     UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    changed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    transition_id  TEXT NULL
);
CREATE INDEX idx_issue_status_history_issue ON issue_status_history (issue_id, changed_at);
CREATE INDEX idx_issue_status_history_to ON issue_status_history (to_status_id);

CREATE TABLE board_column_statuses (
    column_id UUID NOT NULL REFERENCES board_columns (id) ON DELETE CASCADE,
    status_id UUID NOT NULL REFERENCES statuses (id) ON DELETE CASCADE,
    PRIMARY KEY (column_id, status_id)
);
CREATE INDEX idx_board_column_statuses_status ON board_column_statuses (status_id);

-- 2. Helper functions.

-- corelog_legacy_status maps a status category to the legacy enum value.
CREATE FUNCTION corelog_legacy_status(p_category text) RETURNS text
LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE p_category WHEN 'todo' THEN 'open' WHEN 'in_progress' THEN 'in_progress' ELSE 'closed' END
$$;

-- corelog_workflow_template returns the default workflow of a project template. Status ids are
-- slugs; corelog_ensure_workflow replaces them with organization status ids.
CREATE FUNCTION corelog_workflow_template(p_template text) RETURNS jsonb
LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE p_template
        WHEN 'task_tracking' THEN '{"version":1,"initial":"todo","statuses":[{"id":"todo","name":"Por hacer","category":"todo"},{"id":"in_progress","name":"En curso","category":"in_progress"},{"id":"done","name":"Hecho","category":"done"}],"transitions":[{"id":"start","name":"Iniciar","from":["todo"],"to":"in_progress","conditions":[],"validators":[],"post_functions":[],"required_fields":[]},{"id":"pause","name":"Pausar","from":["in_progress"],"to":"todo","conditions":[],"validators":[],"post_functions":[],"required_fields":[]},{"id":"complete","name":"Completar","from":["todo","in_progress"],"to":"done","conditions":[],"validators":[],"post_functions":[{"type":"set_resolution","params":{"resolution":"done"}}],"required_fields":[]},{"id":"reopen","name":"Reabrir","from":["done"],"to":"todo","conditions":[],"validators":[],"post_functions":[{"type":"clear_resolution","params":{}}],"required_fields":[]}]}'::jsonb
        WHEN 'bug_tracking' THEN '{"version":1,"initial":"new","statuses":[{"id":"new","name":"Nuevo","category":"todo"},{"id":"confirmed","name":"Confirmado","category":"todo"},{"id":"in_progress","name":"En curso","category":"in_progress"},{"id":"resolved","name":"Resuelto","category":"done"},{"id":"closed","name":"Cerrado","category":"done"}],"transitions":[{"id":"confirm","name":"Confirmar","from":["new"],"to":"confirmed","conditions":[],"validators":[],"post_functions":[],"required_fields":[]},{"id":"start","name":"Iniciar","from":["new","confirmed"],"to":"in_progress","conditions":[],"validators":[],"post_functions":[],"required_fields":[]},{"id":"resolve","name":"Resolver","from":["in_progress"],"to":"resolved","conditions":[],"validators":[],"post_functions":[{"type":"set_resolution","params":{"resolution":"done"}}],"required_fields":[]},{"id":"close","name":"Cerrar","from":["new","confirmed","in_progress","resolved"],"to":"closed","conditions":[],"validators":[],"post_functions":[{"type":"set_resolution","params":{"resolution":"done"}}],"required_fields":[]},{"id":"reopen","name":"Reabrir","from":["resolved"],"to":"in_progress","conditions":[],"validators":[],"post_functions":[{"type":"clear_resolution","params":{}}],"required_fields":[]}]}'::jsonb
        ELSE '{"version":1,"initial":"todo","statuses":[{"id":"todo","name":"Por hacer","category":"todo"},{"id":"in_progress","name":"En curso","category":"in_progress"},{"id":"in_review","name":"En revisión","category":"in_progress"},{"id":"done","name":"Hecho","category":"done"}],"transitions":[{"id":"start","name":"Iniciar","from":["todo"],"to":"in_progress","conditions":[],"validators":[],"post_functions":[],"required_fields":[]},{"id":"send_to_review","name":"Enviar a revisión","from":["in_progress"],"to":"in_review","conditions":[],"validators":[],"post_functions":[],"required_fields":[]},{"id":"approve","name":"Aprobar","from":["in_review"],"to":"done","conditions":[],"validators":[],"post_functions":[{"type":"set_resolution","params":{"resolution":"done"}}],"required_fields":[]},{"id":"return","name":"Devolver","from":["in_review"],"to":"in_progress","conditions":[],"validators":[],"post_functions":[],"required_fields":[]},{"id":"pause","name":"Pausar","from":["in_progress"],"to":"todo","conditions":[],"validators":[],"post_functions":[],"required_fields":[]},{"id":"complete","name":"Completar","from":["todo","in_progress"],"to":"done","conditions":[],"validators":[],"post_functions":[{"type":"set_resolution","params":{"resolution":"done"}}],"required_fields":[]},{"id":"reopen","name":"Reabrir","from":["done"],"to":"todo","conditions":[],"validators":[],"post_functions":[{"type":"clear_resolution","params":{}}],"required_fields":[]}]}'::jsonb
    END
$$;

-- corelog_ensure_workflow gives a project the workflow of its template when it has none: it creates
-- the missing organization statuses, stores the definition and binds the board columns to the
-- statuses whose legacy mirror matches the column.
CREATE FUNCTION corelog_ensure_workflow(p_project uuid) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_org      uuid;
    v_template text;
    v_tpl      jsonb;
    v_status   jsonb;
    v_map      jsonb := '{}'::jsonb;
    v_id       uuid;
    v_def      jsonb;
BEGIN
    IF EXISTS (SELECT 1 FROM workflows WHERE project_id = p_project) THEN
        RETURN;
    END IF;
    SELECT organization_id, template INTO v_org, v_template FROM projects WHERE id = p_project;
    IF NOT FOUND THEN
        RETURN;
    END IF;
    v_tpl := corelog_workflow_template(v_template);
    FOR v_status IN SELECT value FROM jsonb_array_elements(v_tpl -> 'statuses') LOOP
        INSERT INTO statuses (organization_id, name, category, position)
        VALUES (v_org, v_status ->> 'name', v_status ->> 'category',
                COALESCE((SELECT max(position) + 1 FROM statuses WHERE organization_id = v_org), 0))
        ON CONFLICT (organization_id, (lower(name))) DO NOTHING;
        SELECT id INTO v_id FROM statuses WHERE organization_id = v_org AND lower(name) = lower(v_status ->> 'name');
        v_map := v_map || jsonb_build_object(v_status ->> 'id', v_id);
    END LOOP;

    v_def := jsonb_build_object(
        'version', v_tpl -> 'version',
        'initial', v_map ->> (v_tpl ->> 'initial'),
        'statuses', (
            SELECT jsonb_agg(jsonb_build_object('id', st.id, 'name', st.name, 'category', st.category) ORDER BY t.ord)
            FROM jsonb_array_elements(v_tpl -> 'statuses') WITH ORDINALITY AS t(s, ord)
            JOIN statuses st ON st.id = (v_map ->> (t.s ->> 'id'))::uuid
        ),
        'transitions', (
            SELECT jsonb_agg(
                jsonb_set(
                    jsonb_set(t.x, '{to}', to_jsonb(v_map ->> (t.x ->> 'to'))),
                    '{from}',
                    COALESCE((SELECT jsonb_agg(to_jsonb(v_map ->> f.value) ORDER BY f.ord)
                              FROM jsonb_array_elements_text(t.x -> 'from') WITH ORDINALITY AS f(value, ord)), '[]'::jsonb)
                ) ORDER BY t.ord)
            FROM jsonb_array_elements(v_tpl -> 'transitions') WITH ORDINALITY AS t(x, ord)
        )
    );
    INSERT INTO workflows (project_id, definition, version) VALUES (p_project, v_def, 1);

    INSERT INTO board_column_statuses (column_id, status_id)
    SELECT c.id, st.id
    FROM boards b
    JOIN board_columns c ON c.board_id = b.id
    JOIN jsonb_array_elements(v_def -> 'statuses') AS s(value) ON true
    JOIN statuses st ON st.id = (s.value ->> 'id')::uuid
    WHERE b.project_id = p_project AND c.status = corelog_legacy_status(st.category)
    ON CONFLICT DO NOTHING;
END
$$;

-- corelog_status_for_legacy picks the status of the project workflow that stands for a legacy value.
CREATE FUNCTION corelog_status_for_legacy(p_project uuid, p_legacy text) RETURNS uuid
LANGUAGE plpgsql AS $$
DECLARE
    d    jsonb;
    v_id uuid;
BEGIN
    PERFORM corelog_ensure_workflow(p_project);
    SELECT definition INTO d FROM workflows WHERE project_id = p_project;
    IF d IS NULL THEN
        RETURN NULL;
    END IF;
    SELECT (t.s ->> 'id')::uuid INTO v_id
    FROM jsonb_array_elements(d -> 'statuses') WITH ORDINALITY AS t(s, ord)
    WHERE CASE p_legacy
            WHEN 'open' THEN t.s ->> 'category' = 'todo'
            WHEN 'in_progress' THEN t.s ->> 'category' = 'in_progress'
            ELSE t.s ->> 'category' = 'done'
          END
    ORDER BY CASE
                 WHEN p_legacy = 'open' AND t.s ->> 'id' = d ->> 'initial' THEN 0
                 WHEN p_legacy = 'resolved' AND lower(t.s ->> 'name') IN ('resuelto', 'resuelta', 'resolved') THEN 0
                 WHEN p_legacy = 'closed' AND lower(t.s ->> 'name') IN ('cerrado', 'cerrada', 'closed') THEN 0
                 ELSE 1
             END, t.ord
    LIMIT 1;
    RETURN COALESCE(v_id, (d ->> 'initial')::uuid);
END
$$;

-- 3. Backfill.
SELECT corelog_ensure_workflow(id) FROM projects;

UPDATE issues i
SET status_id = m.sid
FROM (SELECT DISTINCT project_id, status, corelog_status_for_legacy(project_id, status) AS sid FROM issues) m
WHERE i.project_id = m.project_id AND i.status = m.status;

-- Legacy "resolved" rows now mirror their done status, which is "closed".
UPDATE issues i
SET status = corelog_legacy_status(st.category)
FROM statuses st
WHERE st.id = i.status_id AND i.status <> corelog_legacy_status(st.category);

INSERT INTO issue_status_history (issue_id, from_status_id, to_status_id, changed_by, changed_at)
SELECT i.id, NULL, (w.definition ->> 'initial')::uuid, i.reporter_id, i.created_at
FROM issues i JOIN workflows w ON w.project_id = i.project_id;

INSERT INTO issue_status_history (issue_id, from_status_id, to_status_id, changed_by, changed_at)
SELECT i.id, (w.definition ->> 'initial')::uuid, i.status_id, NULL, GREATEST(i.updated_at, i.created_at)
FROM issues i JOIN workflows w ON w.project_id = i.project_id
WHERE i.status_id <> (w.definition ->> 'initial')::uuid;

ALTER TABLE issues ALTER COLUMN status_id SET NOT NULL;
CREATE INDEX idx_issues_status_id ON issues (status_id);

-- 4. Keep status and status_id in sync for every writer.
CREATE FUNCTION corelog_sync_issue_status() RETURNS trigger
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
        NEW.status_id := corelog_status_for_legacy(NEW.project_id, NEW.status);
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER trg_issues_sync_status
    BEFORE INSERT OR UPDATE OF status, status_id, project_id ON issues
    FOR EACH ROW EXECUTE FUNCTION corelog_sync_issue_status();
