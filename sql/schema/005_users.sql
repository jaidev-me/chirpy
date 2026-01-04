-- +goose up
ALTER TABLE users
ADD is_chirpy_red BOOLEAN NOT NULL DEFAULT false;

-- +goose down
ALTER TABLE users
DROP COLUMN is_chirpy_red;