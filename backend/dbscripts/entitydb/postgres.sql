-- Table to store Organization Units
CREATE TABLE "ORGANIZATION_UNIT" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    OU_ID           VARCHAR(36) NOT NULL,
    PARENT_ID       VARCHAR(36),
    PROJECT_ID      VARCHAR(36),
    HANDLE          VARCHAR(100)        NOT NULL,
    NAME            VARCHAR(100)        NOT NULL,
    DESCRIPTION     VARCHAR(255),
    METADATA         JSONB,
    CREATED_AT      TIMESTAMPTZ NOT NULL,
    UPDATED_AT      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (DEPLOYMENT_ID, OU_ID)
);

-- Composite index for handle-based OU lookups
CREATE INDEX idx_ou_handle_parent ON "ORGANIZATION_UNIT" (DEPLOYMENT_ID, HANDLE, PARENT_ID);

-- Composite index for listing the root organization units a project owns
CREATE INDEX idx_ou_project ON "ORGANIZATION_UNIT" (DEPLOYMENT_ID, PROJECT_ID);

-- Table to store Projects. Every project includes the system organization unit.
CREATE TABLE "PROJECT" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    ID              VARCHAR(36)  NOT NULL,
    HANDLE          VARCHAR(100) NOT NULL,
    DISPLAY_NAME    VARCHAR(100) NOT NULL,
    DESCRIPTION     VARCHAR(255),
    CREATED_AT      TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Unique index for project handle lookups and conflict checks
CREATE UNIQUE INDEX idx_project_handle ON "PROJECT" (DEPLOYMENT_ID, HANDLE);

-- Table to store Entities (unified identity principals: users, applications, agents)
CREATE TABLE "ENTITY" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    ID                  VARCHAR(36) NOT NULL,
    CATEGORY            VARCHAR(50)  NOT NULL,
    TYPE                VARCHAR(100) NOT NULL,
    OU_ID               VARCHAR(36)  NOT NULL,
    PROJECT_ID          VARCHAR(36),
    ATTRIBUTES          JSONB,
    SYSTEM_ATTRIBUTES   JSONB,
    CREDENTIALS         JSONB,
    SYSTEM_CREDENTIALS  JSONB,
    CREATED_AT          TIMESTAMPTZ NOT NULL,
    UPDATED_AT          TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Composite index for category-based entity listing
CREATE INDEX idx_entity_category_deployment ON "ENTITY" (DEPLOYMENT_ID, CATEGORY);

-- Composite index for OU-based entity listing
CREATE INDEX idx_entity_ou_deployment ON "ENTITY" (DEPLOYMENT_ID, OU_ID);

-- Composite index for project-based entity listing and for finding whether a project has entities
CREATE INDEX idx_entity_project ON "ENTITY" (DEPLOYMENT_ID, PROJECT_ID, CATEGORY);

-- Expression index for looking up agents by their owner
CREATE INDEX idx_entity_owner ON "ENTITY" (DEPLOYMENT_ID, (SYSTEM_ATTRIBUTES->>'owner'));

-- Table to store Groups
CREATE TABLE "GROUP" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    ID              VARCHAR(36) NOT NULL,
    OU_ID           VARCHAR(36)        NOT NULL,
    NAME            VARCHAR(50)        NOT NULL,
    DESCRIPTION     VARCHAR(255),
    CREATED_AT      TIMESTAMPTZ NOT NULL,
    UPDATED_AT      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Composite index for name conflict checks within an OU
CREATE INDEX idx_group_name_ou_deployment ON "GROUP" (DEPLOYMENT_ID, OU_ID, NAME);

-- Table to store Group member assignments
CREATE TABLE "GROUP_MEMBER_REFERENCE" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    GROUP_ID    VARCHAR(36) NOT NULL,
    MEMBER_TYPE VARCHAR(6)  NOT NULL CHECK (MEMBER_TYPE IN ('entity', 'group')),
    MEMBER_ID   VARCHAR(36) NOT NULL,
    CREATED_AT  TIMESTAMPTZ NOT NULL,
    UPDATED_AT  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (GROUP_ID, MEMBER_TYPE, MEMBER_ID, DEPLOYMENT_ID)
);

-- Table to store indexed entity identifiers for fast lookups (authentication, identification)
-- An entity may hold multiple values for the same NAME (e.g. more than one email address).
CREATE TABLE "ENTITY_IDENTIFIER" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    ENTITY_ID       VARCHAR(36)  NOT NULL,
    NAME            VARCHAR(255) NOT NULL,
    VALUE           TEXT         NOT NULL,
    SOURCE          VARCHAR(50)  NOT NULL,
    CREATED_AT      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (ENTITY_ID, DEPLOYMENT_ID, NAME, VALUE),
    FOREIGN KEY (DEPLOYMENT_ID, ENTITY_ID)
        REFERENCES "ENTITY" (DEPLOYMENT_ID, ID) ON DELETE CASCADE
);

-- Index for fast identifier lookups (primary use case for authentication)
CREATE INDEX idx_entity_identifier_lookup ON "ENTITY_IDENTIFIER" (NAME, VALUE);
