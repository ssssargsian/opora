-- +goose Up
INSERT INTO permissions(code, description)
VALUES ('organization.update', 'Update organization settings')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions(role_id, permission_code)
SELECT id, 'organization.update'
FROM roles
WHERE role_key = 'organization_admin'
ON CONFLICT DO NOTHING;

ALTER TABLE users
    ADD COLUMN last_name text,
    ADD COLUMN first_name text,
    ADD COLUMN middle_name text;

UPDATE users
SET last_name = split_part(display_name, ' ', 1),
    first_name = CASE
        WHEN position(' ' IN display_name) > 0 THEN substr(display_name, position(' ' IN display_name) + 1)
        ELSE 'Пользователь'
    END;

ALTER TABLE users
    ADD CONSTRAINT users_last_name_length CHECK (last_name IS NULL OR char_length(last_name) BETWEEN 1 AND 100),
    ADD CONSTRAINT users_first_name_length CHECK (first_name IS NULL OR char_length(first_name) BETWEEN 1 AND 100),
    ADD CONSTRAINT users_middle_name_length CHECK (middle_name IS NULL OR char_length(middle_name) BETWEEN 1 AND 100);

CREATE TABLE user_invitations (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    user_id uuid NOT NULL,
    token_hash bytea NOT NULL CHECK (octet_length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    accepted_at timestamptz,
    revoked_at timestamptz,
    CONSTRAINT user_invitations_token_hash_unique UNIQUE (token_hash),
    CONSTRAINT user_invitations_membership_fk
        FOREIGN KEY (organization_id, user_id)
        REFERENCES memberships (organization_id, user_id) ON DELETE CASCADE,
    CONSTRAINT user_invitations_creator_fk
        FOREIGN KEY (organization_id, created_by)
        REFERENCES memberships (organization_id, user_id) ON DELETE RESTRICT,
    CONSTRAINT user_invitations_expiry_after_creation CHECK (expires_at > created_at),
    CONSTRAINT user_invitations_terminal_state CHECK (accepted_at IS NULL OR revoked_at IS NULL)
);

CREATE INDEX user_invitations_pending_user_idx
    ON user_invitations (organization_id, user_id, created_at DESC)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

CREATE INDEX sessions_user_active_idx
    ON sessions (organization_id, user_id)
    WHERE revoked_at IS NULL;

-- +goose Down
DROP INDEX sessions_user_active_idx;
DROP TABLE user_invitations;
ALTER TABLE users
    DROP CONSTRAINT users_middle_name_length,
    DROP CONSTRAINT users_first_name_length,
    DROP CONSTRAINT users_last_name_length,
    DROP COLUMN middle_name,
    DROP COLUMN first_name,
    DROP COLUMN last_name;
DELETE FROM role_permissions WHERE permission_code = 'organization.update';
DELETE FROM permissions WHERE code = 'organization.update';
