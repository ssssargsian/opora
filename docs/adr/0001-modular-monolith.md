# ADR 0001: Modular Monolith

## Context

MVP needs strong domain boundaries without distributed-system operational cost.

## Decision

Use one Go deployment organized into `auth`, `organization`, `user`, `student`, `document`, `access`, and `audit` modules, with one Next.js frontend and one PostgreSQL database.

## Consequences

Transactions and local calls remain simple. Module ownership must be enforced in code review; microservices require a new ADR.
