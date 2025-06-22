-- Drop triggers first
DROP TRIGGER IF EXISTS users_track_username_changes ON users;
DROP TRIGGER IF EXISTS users_update_timestamp ON users;

-- Drop functions
DROP FUNCTION IF EXISTS track_username_changes();
DROP FUNCTION IF EXISTS update_user_timestamp();

-- Drop tables (username_changelog first due to foreign key)
DROP TABLE IF EXISTS username_changelog;
DROP TABLE IF EXISTS users;