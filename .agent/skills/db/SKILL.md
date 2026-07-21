---
name: db
description: Database schema and query conventions for ThunderID. Use when changing schema scripts, defining SQL queries, updating store constants, or reviewing deployment-scoped persistence rules.
---

# Database Schema Design Principles and Conventions

## Logical Database Separation

ThunderID uses four logically separated databases. Each database owns a specific category of data.

| Database (config key) | Responsibility                                                |
|-----------------------|---------------------------------------------------------------|
| `config`              | Identity configuration data Ex: applications, authentication flows, roles, identity providers |
| `runtime_transient`   | Short-lived runtime state: authorization codes, authorization/PAR requests, JTI records, WebAuthn/VCI state, flow contexts |
| `entitydb`                | Identity data: users, groups, indexed user attributes         |
| `runtime_persistent`  | Long-lived operational state that must survive restarts: revoked tokens, SSO sessions, consent records |

Although the databases are logically separated, they share consistent schema design principles documented here.

## Primary Key Strategy

### UUID v7 Identifiers

Tables use UUID v7 as primary key values. UUID v7 provides:

- Global uniqueness across deployments and systems.
- Time-ordering characteristics that improve index performance for insert-heavy workloads.

### Primary Key Column Naming

Primary key columns are named `ID`.

Do not use composite names such as `USER_ID` or `APPLICATION_ID` for a primary key column. Use `ID` consistently.

```sql
-- Correct
CREATE TABLE "APPLICATION" (
    ID VARCHAR(36) PRIMARY KEY,
    ...
);

-- Incorrect
CREATE TABLE APPLICATION (
    APPLICATION_ID VARCHAR(36) PRIMARY KEY,
    ...
);
```

## Association Tables

Association (join) tables that model many-to-many relationships use composite primary keys formed from the relevant foreign key columns. These tables do not include a separate surrogate `ID` column.

```sql
-- Example: role assignments use a composite primary key
CREATE TABLE "ROLE_ASSIGNMENT" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    ROLE_ID         VARCHAR(36) NOT NULL,
    ASSIGNEE_TYPE   VARCHAR(5)  NOT NULL CHECK (ASSIGNEE_TYPE IN ('user', 'group')),
    ASSIGNEE_ID     VARCHAR(36) NOT NULL,
    PRIMARY KEY (ROLE_ID, DEPLOYMENT_ID, ASSIGNEE_TYPE, ASSIGNEE_ID),
    FOREIGN KEY (ROLE_ID) REFERENCES "ROLE" (ID) ON DELETE CASCADE
);
```

This reduces index size, matches natural many-to-many query patterns, and prevents duplicate association rows at the database level.

## Foreign Key Strategy

Foreign keys reference UUID primary keys directly. Do not introduce `INT`-based surrogate identifiers, UUID-to-integer resolution layers, or auto-increment IDs anywhere in the schema — UUID v7 is the only primary key mechanism.

## Multi-Deployment Isolation

### Overview

ThunderID supports multi-deployment scenarios where a single database instance may serve data from multiple independent deployments. The `DEPLOYMENT_ID` column enforces isolation between these deployments.

### Column Requirement

Every table includes a `DEPLOYMENT_ID` column defined as `VARCHAR(255) NOT NULL`.

```sql
CREATE TABLE "IDP" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    NAME VARCHAR(255) NOT NULL,
    ...
);
```

Although UUID v7 identifiers are globally unique, queries must still filter by `DEPLOYMENT_ID` to prevent data from leaking across deployments.

### Query Patterns

`DEPLOYMENT_ID` is the last parameter in all parameterized queries. Follow these patterns consistently.

### Identifier Casing and Quoting

Use uppercase table names wrapped in double quotes in schema scripts and embedded SQL.

- Write table names as `"TABLE_NAME"`, not bare identifiers.
- Apply this consistently in `CREATE TABLE`, `CREATE INDEX ... ON`, `FOREIGN KEY ... REFERENCES`, and all `SELECT` / `INSERT` / `UPDATE` / `DELETE` statements.
- Keep Go query strings aligned with the schema scripts; do not mix quoted uppercase names with unquoted identifiers for the same table.
- This avoids PostgreSQL case-folding surprises and keeps reserved-word tables such as `"ROLE"` and `"GROUP"` consistent with the rest of the schema.

#### INSERT

Add `DEPLOYMENT_ID` as the last column in the column list and the last parameter in `VALUES`.

```sql
INSERT INTO "IDP" (ID, NAME, DESCRIPTION, TYPE, PROPERTIES, DEPLOYMENT_ID)
VALUES ($1, $2, $3, $4, $5, $6)
```

#### SELECT

Add `AND DEPLOYMENT_ID = $N` as the final condition in the `WHERE` clause.

```sql
SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES
FROM "IDP"
WHERE ID = $1 AND DEPLOYMENT_ID = $2
```

#### UPDATE

Add `AND DEPLOYMENT_ID = $N` as the last condition in the `WHERE` clause.

```sql
UPDATE "IDP"
SET NAME = $2, DESCRIPTION = $3, TYPE = $4, PROPERTIES = $5
WHERE ID = $1 AND DEPLOYMENT_ID = $6
```

#### DELETE

Add `AND DEPLOYMENT_ID = $N` as the last condition in the `WHERE` clause.

```sql
DELETE FROM "IDP"
WHERE ID = $1 AND DEPLOYMENT_ID = $2
```

#### JOIN Queries

Include `DEPLOYMENT_ID` in `JOIN` conditions and `WHERE` clauses.

```sql
SELECT f.ID, f.HANDLE, f.NAME, fv.NODES
FROM "FLOW" f
INNER JOIN "FLOW_VERSION" fv
    ON f.ID = fv.FLOW_ID
    AND f.DEPLOYMENT_ID = fv.DEPLOYMENT_ID
    AND f.ACTIVE_VERSION = fv.VERSION
WHERE f.ID = $1 AND f.DEPLOYMENT_ID = $2
```

## Indexing Philosophy

Indexes match real query patterns. When defining or revising indexes: review each table's queries, identify missing or inefficient indexes, optimize existing ones (including composite primary keys), add composite indexes for common patterns, and update both the PostgreSQL and SQLite schema scripts.

### Composite Indexes

Composite indexes should place `DEPLOYMENT_ID` first when the query always filters by deployment. This allows the index to be used for deployment-scoped queries even when additional columns are not included.

```sql
-- Composite index for deployment + OU-based lookups
CREATE INDEX idx_user_ou_deployment ON "USER" (DEPLOYMENT_ID, OU_ID);
```

### Expiry Indexes

Tables in `runtime_transient` that include an `EXPIRY_TIME` column should have a dedicated index on that column to support efficient cleanup queries.

```sql
CREATE INDEX idx_authz_code_expiry_time ON "AUTHORIZATION_CODE" (EXPIRY_TIME);
```

## Runtime-transient Database Expiry Handling

Use these rules for all temporary runtime tables in `runtime_transient`.

### Agent Rules

1. Treat runtime records as temporary; they must expire and be removable.
2. Every runtime table must include an `EXPIRY_TIME` column.
3. Read queries must return only non-expired rows.
4. Cleanup jobs must delete expired rows regularly.
5. For association tables, if the foreign key to the owning runtime record uses `ON DELETE CASCADE`, deleting an expired owner row also removes related association rows automatically.
6. An association table does not require its own `EXPIRY_TIME` column unless the association has an independent expiry lifecycle.
7. When runtime tables are added, removed, or renamed, update both cleanup artifacts: `backend/dbscripts/runtime-transient/postgres-cleanup.sql` and `backend/scripts/cleanup_runtime_transient_db.sh`.

### Expiry Column

Required column in each runtime table:

```sql
EXPIRY_TIME TIMESTAMP NOT NULL
```

### Read Query Pattern

When selecting runtime data, compare `EXPIRY_TIME` with current time and keep `DEPLOYMENT_ID` as the last parameter:

```sql
SELECT AUTH_ID, REQUEST_DATA, EXPIRY_TIME
FROM "AUTHORIZATION_REQUEST"
WHERE AUTH_ID = $1 AND EXPIRY_TIME > $2 AND DEPLOYMENT_ID = $3
```

### Cleanup Mechanism

Use the existing cleanup artifacts in this repository:

- `backend/dbscripts/runtime-transient/postgres-cleanup.sql`: defines the PostgreSQL stored procedure `cleanup_expired_runtime_transient_data` (UTC-based cleanup).
- `backend/scripts/cleanup_runtime_transient_db.sh`: provides scheduled/manual cleanup support for PostgreSQL and SQLite.

Keep these two files in sync with the current set of runtime tables.

```sql
CREATE OR REPLACE PROCEDURE cleanup_expired_runtime_transient_data()
LANGUAGE plpgsql
AS $$
DECLARE
    v_now TIMESTAMP := NOW() AT TIME ZONE 'UTC';
BEGIN
    DELETE FROM "FLOW_CONTEXT"          WHERE EXPIRY_TIME < v_now;
    DELETE FROM "AUTHORIZATION_CODE"    WHERE EXPIRY_TIME < v_now;
    DELETE FROM "AUTHORIZATION_REQUEST" WHERE EXPIRY_TIME < v_now;
    DELETE FROM "WEBAUTHN_SESSION"      WHERE EXPIRY_TIME < v_now;
    DELETE FROM "ATTRIBUTE_CACHE"       WHERE EXPIRY_TIME < v_now;
END;
$$;
```

## Defining Queries

### DBQuery

Queries are defined as `DBQuery` values from `internal/system/database/model`. Each query requires a unique `ID` for traceability.

```go
var queryGetIDPByID = model.DBQuery{
    ID:    "IPQ-IDP_MGT-02",
    Query: "SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES FROM \"IDP\" WHERE ID = $1 AND DEPLOYMENT_ID = $2",
}
```

### Database-Specific Queries

When query syntax differs between PostgreSQL and SQLite, define both variants using the `Query` and `SQLiteQuery` fields on `DBQuery`.

```go
var queryUpsertTranslation = dbmodel.DBQuery{
    ID:    "I18N-06",
    Query: `INSERT INTO "TRANSLATION" (MESSAGE_KEY, LANGUAGE_CODE, NAMESPACE, VALUE, DEPLOYMENT_ID)
            VALUES ($1, $2, $3, $4, $5)
            ON CONFLICT (DEPLOYMENT_ID, NAMESPACE, MESSAGE_KEY, LANGUAGE_CODE)
            DO UPDATE SET VALUE = excluded.VALUE, UPDATED_AT = NOW()`,
    SQLiteQuery: `INSERT INTO "TRANSLATION" (MESSAGE_KEY, LANGUAGE_CODE, NAMESPACE, VALUE, DEPLOYMENT_ID)
                  VALUES ($1, $2, $3, $4, $5)
                  ON CONFLICT (DEPLOYMENT_ID, NAMESPACE, MESSAGE_KEY, LANGUAGE_CODE)
                  DO UPDATE SET VALUE = excluded.VALUE, UPDATED_AT = datetime('now')`,
}
```

### Query ID Naming Convention

Query IDs follow the pattern `<PREFIX>-<DOMAIN>_MGT-<SEQUENCE>`, for example:

- `IPQ-IDP_MGT-02` — identity provider query, sequence 2.
- `ASQ-USER_MGT-04` — user management query, sequence 4.
- `AZQ-ARS-02` — authorization request store query, sequence 2.

Use a consistent prefix per store and increment the sequence number for each new query in that store.

## Schema Script Conventions

- Maintain separate schema scripts for PostgreSQL (`postgres.sql`) and SQLite (`sqlite.sql`) in each database directory under `backend/dbscripts/`.
- Apply schema changes to both scripts unless a feature is explicitly PostgreSQL-only.
- Add inline comments above each table and index definition explaining its purpose.
- Place indexes immediately after the table they support.

## Quick Reference

| Agent Check | Required Convention |
|-------------|---------------------|
| Primary key format | Use UUID v7 values. |
| Primary key column name | Use `ID` (do not use entity-specific PK names like `USER_ID`). |
| Association table key strategy | Use composite primary key from foreign key columns; do not add surrogate `ID`. |
| Foreign key type | Reference UUID keys directly; do not introduce integer key layers. |
| Auto-increment usage | Do not use auto-increment IDs. |
| Multi-deployment isolation | Include `DEPLOYMENT_ID VARCHAR(255) NOT NULL` in every table. |
| Query parameter order | Keep `DEPLOYMENT_ID` as the last parameter in parameterized queries. |
| Runtime table expiry column | For runtime owner tables, require `EXPIRY_TIME TIMESTAMP NOT NULL`. |
| Association table expiry column | Omit `EXPIRY_TIME` when lifecycle is inherited via `ON DELETE CASCADE`; add it only if association rows expire independently. |
| Expired data cleanup | Use `backend/dbscripts/runtime-transient/postgres-cleanup.sql` and `backend/scripts/cleanup_runtime_transient_db.sh`; keep both updated when runtime tables change. |
| Query declaration format | Define queries as `DBQuery` values with unique query IDs. |
| Table identifier format | Use uppercase table names in double quotes in schema scripts and embedded SQL. |
