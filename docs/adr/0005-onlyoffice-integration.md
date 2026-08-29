# ADR 0005: ONLYOFFICE Integration

## Context

DOCX editing must occur in the browser while preserving authorization and immutable versions.

## Decision

The backend creates JWT-signed editor configurations after tenant and permission checks. ONLYOFFICE reads a short-lived, signed URL for an exact version and sends JWT-protected callbacks to the API. Successful saves are size- and type-checked, scanned, written as a new S3 object, and committed as a new immutable DocumentVersion with an atomic current-version switch.

## Consequences

The JWT secret stays server-side. Document keys represent the current immutable version. Repeated callbacks are idempotent, and a callback based on a stale version fails rather than overwriting a concurrent edit. Local Docker networking requires separate browser-facing and container-facing API URLs.
