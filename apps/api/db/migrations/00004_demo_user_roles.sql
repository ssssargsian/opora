-- +goose Up
INSERT INTO permissions(code, description)
VALUES ('users.create', 'Create a local organization user')
ON CONFLICT (code) DO NOTHING;

ALTER TABLE roles
    ADD COLUMN default_all_students boolean NOT NULL DEFAULT false;

UPDATE roles SET default_all_students = true
WHERE role_key = 'organization_admin';

INSERT INTO roles (id, organization_id, role_key, name, is_system, default_all_students)
SELECT md5(o.id::text || ':' || role.role_key)::uuid, o.id, role.role_key, role.name, true, false
FROM organizations o
CROSS JOIN (VALUES
    ('psychologist', 'Психолог'),
    ('specialist', 'Специалист'),
    ('viewer', 'Просмотр')
) AS role(role_key, name)
ON CONFLICT (organization_id, role_key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_code)
SELECT r.id, mapping.permission_code
FROM roles r
JOIN (VALUES
    ('psychologist', 'students.list'),
    ('psychologist', 'students.view'),
    ('psychologist', 'students.update'),
    ('psychologist', 'students.create'),
    ('psychologist', 'documents.list'),
    ('psychologist', 'documents.view'),
    ('psychologist', 'documents.download'),
    ('psychologist', 'documents.upload'),
    ('psychologist', 'documents.edit'),
    ('specialist', 'students.list'),
    ('specialist', 'students.view'),
    ('specialist', 'students.create'),
    ('specialist', 'documents.list'),
    ('specialist', 'documents.view'),
    ('specialist', 'documents.download'),
    ('specialist', 'documents.upload'),
    ('specialist', 'documents.edit'),
    ('viewer', 'students.list'),
    ('viewer', 'students.view'),
    ('viewer', 'documents.list'),
    ('viewer', 'documents.view'),
    ('viewer', 'documents.download')
) AS mapping(role_key, permission_code) ON mapping.role_key = r.role_key
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions(role_id, permission_code)
SELECT id, 'users.create' FROM roles WHERE role_key = 'organization_admin'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE role_id IN (
    SELECT id FROM roles WHERE is_system AND role_key IN ('psychologist', 'specialist', 'viewer')
);
DELETE FROM roles
WHERE is_system AND role_key IN ('psychologist', 'specialist', 'viewer');
DELETE FROM role_permissions WHERE permission_code = 'users.create';
DELETE FROM permissions WHERE code = 'users.create';
ALTER TABLE roles DROP COLUMN default_all_students;
