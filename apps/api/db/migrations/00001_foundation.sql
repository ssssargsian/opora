-- +goose Up
CREATE TABLE organizations (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 255),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL CHECK (email = lower(email) AND char_length(email) <= 320),
    password_hash text,
    display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 255),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_password_hash_format CHECK (
        password_hash IS NULL OR password_hash LIKE '$argon2id$%'
    )
);

CREATE TABLE permissions (
    code text PRIMARY KEY,
    description text NOT NULL
);

INSERT INTO permissions (code, description) VALUES
    ('students.list', 'List students within the granted scope'),
    ('students.view', 'View a student'),
    ('students.create', 'Create a student'),
    ('students.update', 'Update a student'),
    ('documents.list', 'List student documents'),
    ('documents.view', 'View document metadata'),
    ('documents.download', 'Download document content'),
    ('documents.upload', 'Upload a document'),
    ('documents.edit', 'Edit a document'),
    ('documents.delete', 'Delete a logical document'),
    ('access.view', 'View access grants'),
    ('access.manage', 'Manage access grants'),
    ('audit.view', 'View audit events'),
    ('users.view', 'View organization users'),
    ('users.invite', 'Invite users'),
    ('users.manage', 'Manage users and roles');

CREATE TABLE roles (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    role_key text NOT NULL CHECK (role_key ~ '^[a-z][a-z0-9_]{1,63}$'),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    is_system boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT roles_organization_key_unique UNIQUE (organization_id, role_key),
    CONSTRAINT roles_organization_id_id_unique UNIQUE (organization_id, id)
);

CREATE TABLE role_permissions (
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_code text NOT NULL REFERENCES permissions(code) ON DELETE RESTRICT,
    PRIMARY KEY (role_id, permission_code)
);

CREATE TABLE memberships (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id uuid NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT memberships_organization_id_user_id_unique UNIQUE (organization_id, user_id),
    CONSTRAINT memberships_role_same_organization_fk
        FOREIGN KEY (organization_id, role_id)
        REFERENCES roles (organization_id, id) ON DELETE RESTRICT
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    user_id uuid NOT NULL,
    token_hash bytea NOT NULL CHECK (octet_length(token_hash) = 32),
    csrf_token_hash bytea NOT NULL CHECK (octet_length(csrf_token_hash) = 32),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sessions_token_hash_unique UNIQUE (token_hash),
    CONSTRAINT sessions_membership_fk
        FOREIGN KEY (organization_id, user_id)
        REFERENCES memberships (organization_id, user_id) ON DELETE CASCADE,
    CONSTRAINT sessions_expiry_after_creation CHECK (expires_at > created_at)
);

CREATE INDEX sessions_active_expiry_idx
    ON sessions (expires_at)
    WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE sessions;
DROP TABLE memberships;
DROP TABLE role_permissions;
DROP TABLE roles;
DROP TABLE permissions;
DROP TABLE users;
DROP TABLE organizations;
