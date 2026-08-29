# ADR 0004: S3 Document Storage

## Context

Large, sensitive files do not belong in PostgreSQL and production must not depend on MinIO-specific behavior.

## Decision

Store immutable document versions in a private S3-compatible bucket using opaque UUID-based keys. MinIO is development-only. Database rows contain metadata and hashes, not file bytes.

## Consequences

Downloads require backend authorization and short-lived signed access. Upload completion must coordinate object creation and database state without exposing unscanned files.
