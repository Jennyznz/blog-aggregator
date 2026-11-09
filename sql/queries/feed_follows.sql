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
INNER JOIN feeds ON feed_follows_insert.feed_id = feed.id;

