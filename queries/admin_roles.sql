-- name: GetAdminRoles :many
SELECT role_id FROM admin_roles
WHERE guild_id = $1;

-- name: AddAdminRole :exec
INSERT INTO admin_roles (guild_id, role_id, added_by)
VALUES ($1, $2, $3)
ON CONFLICT (guild_id, role_id) DO NOTHING;

-- name: RemoveAdminRole :exec
DELETE FROM admin_roles
WHERE guild_id = $1 AND role_id = $2;

-- name: GetAllAdminRoles :many
SELECT guild_id, role_id FROM admin_roles
ORDER BY guild_id, role_id;