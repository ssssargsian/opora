# ADR 0006: Authorization Model

## Context

School roles alone are too broad because specialists may work with only assigned students.

## Decision

Roles map to typed permissions. A centralized backend service first checks organization equality and permission, then applies additive student grants (`view`, `upload`, `edit`, `download`) unless the actor has organization-wide student scope. Restricted documents remain outside broad organization scope and will require an explicit grant when document access is implemented.

## Consequences

Role names never drive business logic. Queries must filter list results to the same scope, and every authorization change requires negative cross-tenant and insufficient-grant tests.
