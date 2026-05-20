-- Table to store Organization Units
CREATE TABLE "ORGANIZATION_UNIT" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    OU_ID       VARCHAR(36) PRIMARY KEY,
    PARENT_ID   VARCHAR(36),
    HANDLE      VARCHAR(100)        NOT NULL,
    NAME        VARCHAR(100)        NOT NULL,
    DESCRIPTION VARCHAR(255),
    THEME_ID    VARCHAR(36),
    LAYOUT_ID   VARCHAR(36),
    METADATA     TEXT,
    CREATED_AT  TEXT NOT NULL,
    UPDATED_AT  TEXT NOT NULL
);

-- Composite index for handle-based OU lookups (queryGetRootOrganizationUnitByHandle, queryGetOrganizationUnitByHandle)
CREATE INDEX idx_ou_handle_parent ON "ORGANIZATION_UNIT" (DEPLOYMENT_ID, HANDLE, PARENT_ID);

-- Table to store Entities (unified identity principals: users, applications, agents)
CREATE TABLE "ENTITY" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    ID                  VARCHAR(36)  PRIMARY KEY,
    CATEGORY            VARCHAR(50)  NOT NULL,
    TYPE                VARCHAR(50)  NOT NULL,
    STATE               VARCHAR(50)  NOT NULL,
    OU_ID               VARCHAR(36)  NOT NULL,
    ATTRIBUTES          TEXT,
    SYSTEM_ATTRIBUTES   TEXT,
    CREDENTIALS         TEXT,
    SYSTEM_CREDENTIALS  TEXT,
    CREATED_AT          TEXT NOT NULL,
    UPDATED_AT          TEXT NOT NULL
);

-- Composite index for category-based entity listing
CREATE INDEX idx_entity_category_deployment ON "ENTITY" (DEPLOYMENT_ID, CATEGORY);

-- Composite index for OU-based entity listing
CREATE INDEX idx_entity_ou_deployment ON "ENTITY" (DEPLOYMENT_ID, OU_ID);

-- Table to store Groups
CREATE TABLE "GROUP" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    ID          VARCHAR(36)        PRIMARY KEY,
    OU_ID       VARCHAR(36)        NOT NULL,
    NAME        VARCHAR(50)        NOT NULL,
    DESCRIPTION VARCHAR(255),
    CREATED_AT  TEXT NOT NULL,
    UPDATED_AT  TEXT NOT NULL
);

-- Composite index for name conflict checks within an OU (QueryCheckGroupNameConflict)
CREATE INDEX idx_group_name_ou_deployment ON "GROUP" (DEPLOYMENT_ID, OU_ID, NAME);

-- Table to store Group member assignments
CREATE TABLE "GROUP_MEMBER_REFERENCE" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    GROUP_ID    VARCHAR(36) NOT NULL,
    MEMBER_TYPE VARCHAR(6)  NOT NULL CHECK (MEMBER_TYPE IN ('entity', 'group')),
    MEMBER_ID   VARCHAR(36) NOT NULL,
    CREATED_AT  TEXT NOT NULL,
    UPDATED_AT  TEXT NOT NULL,
    PRIMARY KEY (GROUP_ID, MEMBER_TYPE, MEMBER_ID, DEPLOYMENT_ID),
    FOREIGN KEY (GROUP_ID) REFERENCES "GROUP" (ID) ON DELETE CASCADE
);

-- Table to store indexed entity identifiers for fast lookups (authentication, identification)
CREATE TABLE "ENTITY_IDENTIFIER" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    ENTITY_ID       VARCHAR(36)  NOT NULL,
    NAME            VARCHAR(255) NOT NULL,
    VALUE           TEXT         NOT NULL,
    SOURCE          VARCHAR(50)  NOT NULL,
    CREATED_AT      TEXT NOT NULL,
    PRIMARY KEY (ENTITY_ID, DEPLOYMENT_ID, NAME),
    FOREIGN KEY (ENTITY_ID) REFERENCES "ENTITY" (ID) ON DELETE CASCADE
);

-- Index for fast identifier lookups (primary use case for authentication)
CREATE INDEX idx_entity_identifier_lookup ON "ENTITY_IDENTIFIER" (NAME, VALUE);
