-- +goose Up
ALTER TABLE document_versions ADD COLUMN source_version_id uuid;

ALTER TABLE document_versions ADD CONSTRAINT document_versions_source_version_fk
    FOREIGN KEY (organization_id, document_id, source_version_id)
    REFERENCES document_versions (organization_id, document_id, id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX document_versions_source_hash_unique
    ON document_versions (document_id, source_version_id, sha256)
    WHERE source_version_id IS NOT NULL;

-- +goose Down
DROP INDEX document_versions_source_hash_unique;
ALTER TABLE document_versions DROP CONSTRAINT document_versions_source_version_fk;
ALTER TABLE document_versions DROP COLUMN source_version_id;
