-- name: RevokeRefreshToken :one
UPDATE refresh_tokens SET expires_at = NOW(), updated_at = NOW() WHERE token = $1 
RETURNING *;