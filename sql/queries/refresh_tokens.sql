-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (
    token,
    created_at,
    updated_at,
    user_id,
    expires_at
) VALUES (
    $1,
    NOW(),
    NOW(),
    $2,
    $3
);

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
    SET revoked_at=NOW()
    WHERE token=$1;

-- name: GetUserFromRefreshToken :one
SELECT user_id, expires_at, revoked_at
FROM refresh_tokens
WHERE token=$1;
