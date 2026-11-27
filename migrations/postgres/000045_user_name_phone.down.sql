DROP TABLE IF EXISTS user_phone_verifications;
DROP TRIGGER IF EXISTS trg_users_sync_name ON users;
DROP FUNCTION IF EXISTS users_sync_name();
DROP FUNCTION IF EXISTS users_split_name(TEXT);
DROP INDEX IF EXISTS uq_users_phone_e164_verified;
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_locale_check,
    DROP CONSTRAINT IF EXISTS users_phone_all_or_none_check,
    DROP CONSTRAINT IF EXISTS users_phone_e164_check,
    DROP CONSTRAINT IF EXISTS users_phone_country_check;
ALTER TABLE users
    DROP COLUMN IF EXISTS locale,
    DROP COLUMN IF EXISTS phone_verified_at,
    DROP COLUMN IF EXISTS phone_e164,
    DROP COLUMN IF EXISTS phone_number,
    DROP COLUMN IF EXISTS phone_country,
    DROP COLUMN IF EXISTS last_name,
    DROP COLUMN IF EXISTS first_name;
