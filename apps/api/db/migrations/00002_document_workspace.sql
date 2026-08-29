-- +goose Up
ALTER TABLE memberships ADD COLUMN all_students boolean NOT NULL DEFAULT false;

CREATE TABLE students (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    last_name text NOT NULL CHECK (char_length(last_name) BETWEEN 1 AND 100),
    first_name text NOT NULL CHECK (char_length(first_name) BETWEEN 1 AND 100),
    middle_name text CHECK (middle_name IS NULL OR char_length(middle_name) BETWEEN 1 AND 100),
    birth_date date,
    class_name text CHECK (class_name IS NULL OR char_length(class_name) <= 32),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT students_organization_id_id_unique UNIQUE (organization_id, id)
);

CREATE INDEX students_organization_name_idx
    ON students (organization_id, last_name, first_name, id);

CREATE TABLE student_access_grants (
    organization_id uuid NOT NULL,
    student_id uuid NOT NULL,
    user_id uuid NOT NULL,
    grant_code text NOT NULL CHECK (grant_code IN ('view', 'upload', 'edit', 'download')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, student_id, user_id, grant_code),
    CONSTRAINT student_access_grants_student_fk
        FOREIGN KEY (organization_id, student_id)
        REFERENCES students (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT student_access_grants_membership_fk
        FOREIGN KEY (organization_id, user_id)
        REFERENCES memberships (organization_id, user_id) ON DELETE CASCADE
);

CREATE TABLE documents (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    student_id uuid NOT NULL,
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 255),
    document_type text CHECK (document_type IS NULL OR char_length(document_type) <= 100),
    confidentiality_level text NOT NULL DEFAULT 'standard'
        CHECK (confidentiality_level IN ('standard', 'restricted')),
    current_version_id uuid,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT documents_organization_id_id_unique UNIQUE (organization_id, id),
    CONSTRAINT documents_student_fk
        FOREIGN KEY (organization_id, student_id)
        REFERENCES students (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT documents_creator_fk
        FOREIGN KEY (organization_id, created_by)
        REFERENCES memberships (organization_id, user_id) ON DELETE RESTRICT
);

CREATE INDEX documents_student_updated_idx
    ON documents (organization_id, student_id, updated_at DESC, id);

CREATE TABLE document_versions (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    document_id uuid NOT NULL,
    version_number integer NOT NULL CHECK (version_number > 0),
    object_key text NOT NULL CHECK (char_length(object_key) BETWEEN 1 AND 1024),
    original_filename text NOT NULL CHECK (char_length(original_filename) BETWEEN 1 AND 255),
    mime_type text NOT NULL CHECK (mime_type IN (
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        'application/pdf'
    )),
    size bigint NOT NULL CHECK (size > 0),
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT document_versions_document_version_unique UNIQUE (document_id, version_number),
    CONSTRAINT document_versions_object_key_unique UNIQUE (object_key),
    CONSTRAINT document_versions_organization_document_id_unique
        UNIQUE (organization_id, document_id, id),
    CONSTRAINT document_versions_document_fk
        FOREIGN KEY (organization_id, document_id)
        REFERENCES documents (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT document_versions_creator_fk
        FOREIGN KEY (organization_id, created_by)
        REFERENCES memberships (organization_id, user_id) ON DELETE RESTRICT
);

ALTER TABLE documents ADD CONSTRAINT documents_current_version_fk
    FOREIGN KEY (organization_id, id, current_version_id)
    REFERENCES document_versions (organization_id, document_id, id)
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE audit_events (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_user_id uuid,
    action text NOT NULL CHECK (char_length(action) BETWEEN 1 AND 100),
    resource_type text NOT NULL CHECK (char_length(resource_type) BETWEEN 1 AND 100),
    resource_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    ip_address inet,
    user_agent text CHECK (user_agent IS NULL OR char_length(user_agent) <= 512),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT audit_events_actor_fk
        FOREIGN KEY (organization_id, actor_user_id)
        REFERENCES memberships (organization_id, user_id) ON DELETE SET NULL (actor_user_id)
);

CREATE INDEX audit_events_organization_created_idx
    ON audit_events (organization_id, created_at DESC, id);

-- +goose Down
DROP TABLE audit_events;
ALTER TABLE documents DROP CONSTRAINT documents_current_version_fk;
DROP TABLE document_versions;
DROP TABLE documents;
DROP TABLE student_access_grants;
DROP TABLE students;
ALTER TABLE memberships DROP COLUMN all_students;
