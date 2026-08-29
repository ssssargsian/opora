# ADR 0002: PostgreSQL And sqlc

## Context

Tenant isolation and document version invariants require explicit, reviewable data access.

## Decision

Use PostgreSQL 18, pgx/v5, sqlc, and goose migrations. Do not use an ORM or SQLite substitutes in tests.

## Consequences

SQL and organization predicates stay visible and type checked. Schema changes require migrations and PostgreSQL integration tests.
