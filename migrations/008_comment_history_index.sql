-- Support chronological comment retrieval for one post, using id as the
-- deterministic tie-breaker when comments share a timestamp.
CREATE INDEX idx_comments_post_history
ON comments (post_id, created_at, id);
