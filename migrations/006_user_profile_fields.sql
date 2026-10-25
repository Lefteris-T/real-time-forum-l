ALTER TABLE users
ADD COLUMN age INTEGER NOT NULL DEFAULT 0
CHECK (age > 0);

ALTER TABLE users
ADD COLUMN gender TEXT NOT NULL DEFAULT ''
CHECK (length(trim(gender)) > 0);

ALTER TABLE users
ADD COLUMN first_name TEXT NOT NULL DEFAULT ''
CHECK (length(trim(first_name)) > 0);

ALTER TABLE users
ADD COLUMN last_name TEXT NOT NULL DEFAULT ''
CHECK (length(trim(last_name)) > 0);