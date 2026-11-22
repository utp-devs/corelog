DROP INDEX IF EXISTS idx_issue_components_component_id;
DROP INDEX IF EXISTS idx_components_project_id;
DROP INDEX IF EXISTS idx_issue_labels_label_id;
DROP INDEX IF EXISTS idx_labels_project_id;

DROP TABLE IF EXISTS issue_components;
DROP TABLE IF EXISTS components;
DROP TABLE IF EXISTS issue_labels;
DROP TABLE IF EXISTS labels;
