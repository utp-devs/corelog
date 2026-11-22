-- Las transiciones se evaluan con roles de proyecto (ADR-003). Se convierte el valor por defecto
-- conservando el comportamiento: agent pasa a lead y contributor, requester pasa a viewer.
ALTER TABLE workflow_transitions ALTER COLUMN allowed_roles SET DEFAULT 'admin,lead,contributor,viewer';

UPDATE workflow_transitions
SET allowed_roles = replace(replace(allowed_roles, 'agent', 'lead,contributor'), 'requester', 'viewer')
WHERE allowed_roles LIKE '%agent%' OR allowed_roles LIKE '%requester%';
