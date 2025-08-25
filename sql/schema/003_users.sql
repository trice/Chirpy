-- +goose Up
ALTER TABLE users
    ADD COLUMN hashed_password TEXT NOT NULL default '';

-- +goose Down
ALTER TABLE users
    DROP COLUMN hashed_password;
