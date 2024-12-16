-- name: DeleteChirp :exec
DELETE FROM chirps WHERE user_id = $1 AND id = $2;