-- name: GetUser :one
SELECT * FROM users 
WHERE user_id = $1;

-- name: GetUserByUsername :many
SELECT * FROM users 
WHERE username ILIKE $1 
ORDER BY last_message_at DESC NULLS LAST;

-- name: UpsertUser :one
INSERT INTO users (
    user_id, 
    username, 
    discriminator, 
    display_name, 
    avatar_hash, 
    bot,
    last_message_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) 
ON CONFLICT (user_id) DO UPDATE SET 
    username = EXCLUDED.username,
    discriminator = EXCLUDED.discriminator,
    display_name = EXCLUDED.display_name,
    avatar_hash = EXCLUDED.avatar_hash,
    bot = EXCLUDED.bot,
    last_message_at = GREATEST(users.last_message_at, EXCLUDED.last_message_at),
    last_updated_at = NOW()
RETURNING *;

-- name: UpdateUserLastMessage :exec
UPDATE users 
SET last_message_at = $2, last_updated_at = NOW()
WHERE user_id = $1;

-- name: GetUsersNeedingRefresh :many
SELECT user_id FROM users 
WHERE last_updated_at < NOW() - INTERVAL '1 hour'
   OR last_updated_at IS NULL
ORDER BY last_updated_at ASC NULLS FIRST
LIMIT $1;

-- name: GetStaleUsers :many
SELECT user_id, username, last_updated_at, last_message_at 
FROM users 
WHERE last_updated_at < NOW() - INTERVAL '24 hours'
   AND (last_message_at IS NULL OR last_message_at < NOW() - INTERVAL '30 days')
ORDER BY last_updated_at ASC
LIMIT $1;

-- name: MarkUserInactive :exec
UPDATE users 
SET is_active = FALSE, last_updated_at = NOW()
WHERE user_id = $1;

-- name: GetActiveUsers :many
SELECT * FROM users 
WHERE is_active = TRUE 
   AND last_message_at > NOW() - INTERVAL '30 days'
ORDER BY last_message_at DESC
LIMIT $1;

-- name: GetUsernameHistory :many
SELECT 
    old_username,
    new_username, 
    old_discriminator,
    new_discriminator,
    changed_at,
    detected_via
FROM username_changelog 
WHERE user_id = $1 
ORDER BY changed_at DESC
LIMIT $2;

-- name: GetRecentUsernameChanges :many
SELECT 
    uc.user_id,
    u.username as current_username,
    uc.old_username,
    uc.new_username,
    uc.changed_at,
    uc.detected_via
FROM username_changelog uc
JOIN users u ON uc.user_id = u.user_id
WHERE uc.changed_at > NOW() - INTERVAL '7 days'
ORDER BY uc.changed_at DESC
LIMIT $1;

-- name: CleanupOldUsernameChanges :exec
DELETE FROM username_changelog 
WHERE changed_at < NOW() - INTERVAL '2 years';

-- name: GetUserProfile :one
SELECT 
    u.user_id,
    u.username,
    u.first_seen_at,
    u.last_message_at,
    COUNT(uc.id) as username_changes
FROM users u
LEFT JOIN username_changelog uc ON u.user_id = uc.user_id
WHERE u.user_id = $1
GROUP BY u.user_id, u.username, u.first_seen_at, u.last_message_at;

-- name: SearchUsersByPattern :many
SELECT * FROM users 
WHERE username ILIKE '%' || $1 || '%'
   OR display_name ILIKE '%' || $1 || '%'
ORDER BY 
    CASE WHEN username ILIKE $1 || '%' THEN 0 ELSE 1 END,
    last_message_at DESC NULLS LAST
LIMIT $2;

-- name: BatchUpsertUser :exec
INSERT INTO users (
    user_id, 
    username, 
    discriminator, 
    display_name, 
    avatar_hash, 
    bot
) VALUES (
    $1, $2, $3, $4, $5, $6
) 
ON CONFLICT (user_id) DO UPDATE SET 
    username = EXCLUDED.username,
    discriminator = EXCLUDED.discriminator,
    display_name = EXCLUDED.display_name,
    avatar_hash = EXCLUDED.avatar_hash,
    bot = EXCLUDED.bot,
    last_updated_at = NOW();

-- name: GetUsersWithoutRecentActivity :many
SELECT user_id, username, last_message_at 
FROM users 
WHERE last_message_at < NOW() - INTERVAL '90 days'
   OR last_message_at IS NULL
ORDER BY last_message_at ASC NULLS FIRST
LIMIT $1;