UPDATE workflow_transitions
SET allowed_roles = replace(replace(replace(allowed_roles, 'lead,contributor', 'agent'), 'viewer', 'requester'), 'lead', 'agent')
WHERE allowed_roles LIKE '%lead%' OR allowed_roles LIKE '%viewer%';

ALTER TABLE workflow_transitions ALTER COLUMN allowed_roles SET DEFAULT 'admin,agent,requester';
