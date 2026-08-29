# Opora Engineering Rules

## Architecture

- Opora is a modular monolith: one Go backend, one Next.js frontend, one PostgreSQL database, and one repository.
- Do not introduce microservices without a reviewed ADR that demonstrates a concrete operational need.
- Backend code is grouped by domain modules (`auth`, `organization`, `user`, `student`, `document`, `access`, `audit`). Platform code must remain narrow and infrastructure-specific.
- Do not create generic `utils`, `helpers`, `common`, or `misc` packages.
- HTTP handlers translate transport concerns only. Business logic belongs to domain services; SQL never runs in handlers.
- OpenAPI 3.1 in `openapi/openapi.yaml` is the public REST contract. Application endpoints live under `/api/v1`; operational health endpoints are the only current exception.

## Data And Tenancy

- PostgreSQL is the source of truth. Use pgx/v5, sqlc, and goose. ORMs are prohibited.
- Every tenant-owned row carries `organization_id`. Every tenant-owned query scopes by `organization_id`, even when an entity UUID is known.
- Use real foreign keys, purposeful unique constraints, and indexes justified by an access pattern.
- Use transactions for multi-row invariants. Creating a document version and switching the current version is one transaction.
- Documents are immutable and versioned. Never overwrite an object or delete an old version as part of an edit.
- Document bytes live only in a private S3-compatible bucket. Buckets and objects must never be public, and object keys must not contain names or original filenames.

## Security And Privacy

- Backend authorization is mandatory for every protected action. Frontend permission checks are UX only.
- Business logic checks typed permissions through the centralized authorization service. Do not branch on role names.
- Tenant isolation is checked before resource permissions. Student grants are additive and explicit.
- Organization-wide scope must not automatically expose documents marked restricted; document access will require an explicit grant for that classification.
- Browser authentication uses opaque server-side sessions in Secure, HttpOnly cookies. Store only hashes of session and CSRF tokens. Never put access tokens in localStorage.
- New passwords use Argon2id. Login errors are non-enumerating and login is rate limited.
- Never log passwords, cookies, session tokens, JWTs, presigned URLs, request bodies, document bytes, or unnecessary student data.
- Do not send student documents or their contents to external AI services or unrelated third parties.
- Secrets come from environment variables and never enter Git.

## Quality Gates

- Authorization changes require positive and negative unit tests. Tenant-bound data access requires PostgreSQL integration tests using testcontainers, never SQLite.
- Run Go format, tests, vet, golangci-lint, and govulncheck; run frontend lint, typecheck, and tests.
- Never weaken a security control, tenant predicate, assertion, lint rule, or test merely to make CI green. Fix the underlying design or implementation.
- Keep dependencies minimal, current, and pinned. Prefer the standard library where it is clear and sufficient.
