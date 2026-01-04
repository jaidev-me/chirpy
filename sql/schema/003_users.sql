-- +goose up
ALTER TABLE users
ADD hashed_password TEXT NOT NULL DEFAULT 'unset';

-- +goose down
ALTER TABLE users
DROP COLUMN hashed_password;