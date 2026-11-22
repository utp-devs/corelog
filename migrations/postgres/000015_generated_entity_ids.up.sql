-- Los identificadores de las entidades los genera el motor de la BD (UUID v4 aleatorio)
-- en lugar de la aplicacion. La aplicacion inserta sin id y recupera el valor con RETURNING.
ALTER TABLE users            ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE teams            ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE projects         ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE labels           ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE components       ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE boards           ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE board_columns    ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE workflows        ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE issues           ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE comments         ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE activities       ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE issue_links      ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE saved_filters    ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE attachments      ALTER COLUMN id SET DEFAULT gen_random_uuid();
