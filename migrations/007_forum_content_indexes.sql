-- Support the stable newest-first feed query and lookups from a category to
-- the posts assigned to it. Categories.name already has a UNIQUE index, and
-- post_categories already has a primary key starting with post_id.
CREATE INDEX idx_posts_feed_order
ON posts (created_at DESC, id DESC);

CREATE INDEX idx_post_categories_category_id
ON post_categories (category_id, post_id);
