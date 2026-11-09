-- name: CreateFeedFollow :one
WITH feed_follows_insert AS (
    INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id) 
    VALUES (
        $1,
        $2,
        $3,
        $4,
        $5
    )
    RETURNING *
)
SELECT 
    feed_follows_insert.*,
    users.name AS user_name,
    feeds.name AS feed_name
FROM feed_follows_insert
    INNER JOIN users ON feed_follows_insert.user_id = users.id
    INNER JOIN feeds ON feed_follows_insert.feed_id = feeds.id;

-- name: GetFeedFollowsForUser :many
SELECT 
    feed_follows.*,
    users.name AS user_name,
    feeds.name AS feed_name
FROM feed_follows 
INNER JOIN users ON feed_follows.user_id = users.id
INNER JOIN feeds ON feed_follows.feed_id = feeds.id
WHERE feed_follows.user_id = $1;

