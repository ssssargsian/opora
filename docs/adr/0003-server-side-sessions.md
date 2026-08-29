# ADR 0003: Server-Side Sessions

## Context

Browser-held bearer tokens increase exposure to script compromise and complicate revocation.

## Decision

Use cryptographically random opaque sessions in Secure, HttpOnly, SameSite cookies. Store SHA-256 token hashes and CSRF token hashes, with explicit expiry and revocation. Passwords use Argon2id.

## Consequences

Every authenticated request requires a database-backed session lookup. Revocation is immediate; OIDC and MFA can be added behind the auth module later.
