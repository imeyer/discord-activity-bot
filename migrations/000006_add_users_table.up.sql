-- Create users table to cache Discord user information
CREATE TABLE IF NOT EXISTS users (
    user_id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    discriminator TEXT DEFAULT '0',
    display_name TEXT,
    avatar_hash TEXT,
    bot BOOLEAN DEFAULT FALSE,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_at TIMESTAMPTZ,
    is_active BOOLEAN DEFAULT TRUE
);

-- Create indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);
CREATE INDEX IF NOT EXISTS idx_users_last_updated ON users (last_updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_users_last_message ON users (last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_users_active ON users (is_active, last_message_at DESC);

-- Create username changelog table to track username changes over time
CREATE TABLE IF NOT EXISTS username_changelog (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    old_username TEXT NOT NULL,
    new_username TEXT NOT NULL,
    old_discriminator TEXT DEFAULT '0',
    new_discriminator TEXT DEFAULT '0',
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    detected_via TEXT NOT NULL DEFAULT 'api' -- 'api', 'message', 'manual'
);

-- Create indexes for changelog
CREATE INDEX IF NOT EXISTS idx_username_changelog_user_id ON username_changelog (user_id, changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_username_changelog_changed_at ON username_changelog (changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_username_changelog_usernames ON username_changelog (old_username, new_username);

-- Add function to automatically update last_updated_at
CREATE OR REPLACE FUNCTION update_user_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to auto-update timestamp
CREATE TRIGGER users_update_timestamp
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_user_timestamp();

-- Add function to automatically create changelog entries
CREATE OR REPLACE FUNCTION track_username_changes()
RETURNS TRIGGER AS $$
BEGIN
    -- Only create changelog entry if username or discriminator actually changed
    IF OLD.username != NEW.username OR OLD.discriminator != NEW.discriminator THEN
        INSERT INTO username_changelog (
            user_id, 
            old_username, 
            new_username, 
            old_discriminator, 
            new_discriminator,
            detected_via
        ) VALUES (
            NEW.user_id,
            OLD.username,
            NEW.username,
            OLD.discriminator,
            NEW.discriminator,
            'api'
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to auto-track username changes
CREATE TRIGGER users_track_username_changes
    AFTER UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION track_username_changes();

-- Add retention policy for username changelog (keep 2 years of history)
-- This will be cleaned up by a background job later
COMMENT ON TABLE username_changelog IS 'Tracks username changes over time. Retention: 2 years';