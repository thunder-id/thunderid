-- Table to store Entity Schemas (user/agent categories)
CREATE TABLE "ENTITY_TYPES" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    ID          VARCHAR(36) PRIMARY KEY,
    CATEGORY    VARCHAR(50) NOT NULL,
    NAME        VARCHAR(100) NOT NULL,
    OU_ID       VARCHAR(36) NOT NULL,
    ALLOW_SELF_REGISTRATION BOOLEAN DEFAULT FALSE NOT NULL,
    SCHEMA_DEF  JSONB NOT NULL,
    SYSTEM_ATTRIBUTES JSONB,
    CREATED_AT  TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (NAME, CATEGORY, DEPLOYMENT_ID)
);

-- Composite index for deployment + category + OU-based entity type lookups
CREATE INDEX idx_entity_schemas_deployment_category_ou ON "ENTITY_TYPES" (DEPLOYMENT_ID, CATEGORY, OU_ID);

-- Table to store Roles
CREATE TABLE "ROLE" (
    DEPLOYMENT_ID           VARCHAR(255) NOT NULL,
    ID                  VARCHAR(36) PRIMARY KEY,
    OU_ID               VARCHAR(36) NOT NULL,
    NAME                VARCHAR(50) NOT NULL,
    DESCRIPTION         VARCHAR(255),
    CREATED_AT          TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT          TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT unique_role_ou_name UNIQUE (OU_ID, NAME, DEPLOYMENT_ID)
);

-- Composite index for deployment + OU lookups (supports UNIQUE constraint checks)
CREATE INDEX idx_role_ou_deployment ON "ROLE" (DEPLOYMENT_ID, OU_ID);

-- Table to store Role permissions
CREATE TABLE "ROLE_PERMISSION" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    ROLE_ID             VARCHAR(36) NOT NULL,
    RESOURCE_SERVER_ID  VARCHAR(36) NOT NULL,
    PERMISSION          VARCHAR(1000) NOT NULL,
    CREATED_AT          TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (ROLE_ID, DEPLOYMENT_ID, RESOURCE_SERVER_ID, PERMISSION),
    FOREIGN KEY (ROLE_ID) REFERENCES "ROLE" (ID) ON DELETE CASCADE
);

-- Index for resource server queries with deployment isolation on ROLE_PERMISSION
CREATE INDEX idx_role_permission_resource_server ON "ROLE_PERMISSION" (RESOURCE_SERVER_ID, DEPLOYMENT_ID);

-- Table to store Role assignments (to entities and groups)
CREATE TABLE "ROLE_ASSIGNMENT" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    ROLE_ID         VARCHAR(36) NOT NULL,
    ASSIGNEE_TYPE   VARCHAR(6)  NOT NULL CHECK (ASSIGNEE_TYPE IN ('entity', 'group')),
    ASSIGNEE_ID     VARCHAR(36) NOT NULL,
    CREATED_AT      TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT      TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (ROLE_ID, DEPLOYMENT_ID, ASSIGNEE_TYPE, ASSIGNEE_ID)
);

-- Table to store theme configurations.
CREATE TABLE "THEME" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    DISPLAY_NAME VARCHAR(255) NOT NULL,
    HANDLE VARCHAR(255) NOT NULL,
    DESCRIPTION VARCHAR(512),
    THEME JSONB NOT NULL,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (DEPLOYMENT_ID, HANDLE)
);

-- Index for deployment isolation on THEME
CREATE INDEX idx_theme_deployment_id ON "THEME" (DEPLOYMENT_ID);

-- Unique index for theme handle per deployment
CREATE UNIQUE INDEX idx_theme_handle_deployment ON "THEME" (HANDLE, DEPLOYMENT_ID);

-- Table to store layout configurations.
CREATE TABLE "LAYOUT" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    DISPLAY_NAME VARCHAR(255) NOT NULL,
    HANDLE VARCHAR(255) NOT NULL,
    DESCRIPTION VARCHAR(512),
    LAYOUT JSONB NOT NULL,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (DEPLOYMENT_ID, HANDLE)
);

-- Index for deployment isolation on LAYOUT
CREATE INDEX idx_layout_deployment_id ON "LAYOUT" (DEPLOYMENT_ID);

-- Unique index for layout handle per deployment
CREATE UNIQUE INDEX idx_layout_handle_deployment ON "LAYOUT" (HANDLE, DEPLOYMENT_ID);

-- Table to store inbound client configurations for an entity.
CREATE TABLE "INBOUND_CLIENT" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ENTITY_ID VARCHAR(36) PRIMARY KEY,
    AUTH_FLOW_ID VARCHAR(100) NOT NULL,
    REGISTRATION_FLOW_ID VARCHAR(100),
    IS_REGISTRATION_FLOW_ENABLED CHAR(1) DEFAULT '1',
    RECOVERY_FLOW_ID VARCHAR(100),
    IS_RECOVERY_FLOW_ENABLED CHAR(1) DEFAULT '0',
    SIGNOUT_FLOW_ID VARCHAR(100),
    THEME_ID VARCHAR(36),
    LAYOUT_ID VARCHAR(36),
    PROPERTIES JSONB
);

-- Index for efficient lookups by theme.
CREATE INDEX idx_inbound_client_theme_id ON "INBOUND_CLIENT"(THEME_ID);

-- Index for efficient lookups by layout.
CREATE INDEX idx_inbound_client_layout_id ON "INBOUND_CLIENT"(LAYOUT_ID);

-- Table to store OAuth inbound profile for an entity.
CREATE TABLE "OAUTH_INBOUND_PROFILE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ENTITY_ID VARCHAR(36) NOT NULL,
    OAUTH_CONFIG JSONB,
    PRIMARY KEY (ENTITY_ID, DEPLOYMENT_ID),
    FOREIGN KEY (ENTITY_ID) REFERENCES "INBOUND_CLIENT"(ENTITY_ID) ON DELETE CASCADE
);

-- Table to store identity providers.
CREATE TABLE "IDP" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    NAME VARCHAR(255) NOT NULL,
    DESCRIPTION VARCHAR(500),
    TYPE VARCHAR(20) NOT NULL,
    PROPERTIES JSONB,
    ATTRIBUTE_CONFIGURATION JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW()
);

-- Composite index for name-based IDP lookups
CREATE INDEX idx_idp_name_deployment ON "IDP" (DEPLOYMENT_ID, NAME);

-- Expression index for issuer-based IDP lookups
CREATE INDEX idx_idp_issuer ON "IDP" (DEPLOYMENT_ID, (PROPERTIES->'issuer'->>'value'));

-- Table to store notification senders.
CREATE TABLE "NOTIFICATION_SENDER" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    NAME VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    DESCRIPTION VARCHAR(500),
    TYPE VARCHAR(20) NOT NULL,
    PROVIDER VARCHAR(20) NOT NULL,
    PROPERTIES JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW()
);

-- Composite index for name-based notification sender lookups
CREATE INDEX idx_notification_sender_name_deployment ON "NOTIFICATION_SENDER" (DEPLOYMENT_ID, NAME);

-- Table to store certificates associated with various entities.
CREATE TABLE "CERTIFICATE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    REF_TYPE VARCHAR(20) NOT NULL,
    REF_ID VARCHAR(36) NOT NULL,
    TYPE VARCHAR(20) NOT NULL,
    VALUE TEXT NOT NULL,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (REF_TYPE, REF_ID, DEPLOYMENT_ID)
);

-- Table to store resource servers.
CREATE TABLE "RESOURCE_SERVER" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    OU_ID VARCHAR(36) NOT NULL,
    NAME VARCHAR(100) NOT NULL,
    DESCRIPTION TEXT,
    IDENTIFIER VARCHAR(2048) NOT NULL,
    TYPE VARCHAR(20) CHECK (TYPE IS NULL OR TYPE IN ('API', 'MCP', 'CUSTOM')),
    PROPERTIES JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (OU_ID, NAME, DEPLOYMENT_ID)
);

-- Composite index for name-based resource server lookups
CREATE INDEX idx_resource_server_name_deployment ON "RESOURCE_SERVER" (DEPLOYMENT_ID, NAME);

-- Unique constraint: Resource server identifier must be unique per deployment
CREATE UNIQUE INDEX uq_resource_server_identifier
    ON "RESOURCE_SERVER"(IDENTIFIER, DEPLOYMENT_ID);

-- Table to store resources within resource servers.
CREATE TABLE "RESOURCE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    RESOURCE_SERVER_ID VARCHAR(36) NOT NULL,
    PARENT_RESOURCE_ID VARCHAR(36),
    NAME VARCHAR(100) NOT NULL,
    HANDLE VARCHAR(100) NOT NULL,
    DESCRIPTION TEXT,
    PERMISSION VARCHAR(1000) NOT NULL,
    PROPERTIES JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),

    FOREIGN KEY (RESOURCE_SERVER_ID)
        REFERENCES "RESOURCE_SERVER"(ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    FOREIGN KEY (PARENT_RESOURCE_ID)
        REFERENCES "RESOURCE"(ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
);

-- Composite index for resource server + deployment queries (list, count, and handle checks)
CREATE INDEX idx_resource_server_deployment ON "RESOURCE" (RESOURCE_SERVER_ID, DEPLOYMENT_ID);

-- Unique constraint: Resource handle must be unique under the same parent per deployment
CREATE UNIQUE INDEX uq_resource_handle_with_parent
    ON "RESOURCE"(RESOURCE_SERVER_ID, PARENT_RESOURCE_ID, HANDLE, DEPLOYMENT_ID)
    WHERE PARENT_RESOURCE_ID IS NOT NULL;

-- Unique constraint: Root-level resource handles must be unique per resource server per deployment
CREATE UNIQUE INDEX uq_resource_handle_null_parent
    ON "RESOURCE"(RESOURCE_SERVER_ID, HANDLE, DEPLOYMENT_ID)
    WHERE PARENT_RESOURCE_ID IS NULL;

-- Table to store actions at resource server or resource level.
CREATE TABLE "ACTION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    RESOURCE_SERVER_ID VARCHAR(36) NOT NULL,
    RESOURCE_ID VARCHAR(36),
    NAME VARCHAR(100) NOT NULL,
    HANDLE VARCHAR(100) NOT NULL,
    DESCRIPTION TEXT,
    PERMISSION VARCHAR(1000) NOT NULL,
    PROPERTIES JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),

    FOREIGN KEY (RESOURCE_SERVER_ID)
        REFERENCES "RESOURCE_SERVER"(ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    FOREIGN KEY (RESOURCE_ID)
        REFERENCES "RESOURCE"(ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
);

-- Composite index for action list/count queries filtered by resource server + deployment + resource
CREATE INDEX idx_action_server_deployment ON "ACTION" (RESOURCE_SERVER_ID, DEPLOYMENT_ID, RESOURCE_ID);

-- Unique constraint: Server-level action handles must be unique per resource server per deployment
CREATE UNIQUE INDEX uq_action_server_handle
    ON "ACTION"(RESOURCE_SERVER_ID, HANDLE, DEPLOYMENT_ID)
    WHERE RESOURCE_ID IS NULL;

-- Unique constraint: Resource-level action handles must be unique per resource per deployment
CREATE UNIQUE INDEX uq_action_resource_handle
    ON "ACTION"(RESOURCE_ID, HANDLE, DEPLOYMENT_ID)
    WHERE RESOURCE_ID IS NOT NULL;

-- Table to store active flow definitions
CREATE TABLE "FLOW" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    HANDLE VARCHAR(100) NOT NULL,
    NAME VARCHAR(100) NOT NULL,
    FLOW_TYPE VARCHAR(50) NOT NULL,
    ACTIVE_VERSION INTEGER NOT NULL,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (HANDLE, FLOW_TYPE, DEPLOYMENT_ID)
);

-- Composite index for flow type + deployment queries
CREATE INDEX idx_flow_type_deployment ON "FLOW" (DEPLOYMENT_ID, FLOW_TYPE);

-- Table to store flow version history
CREATE TABLE "FLOW_VERSION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    FLOW_ID VARCHAR(36) NOT NULL,
    VERSION INTEGER NOT NULL,
    NODES JSONB NOT NULL,
    INTERCEPTORS JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (FLOW_ID, VERSION, DEPLOYMENT_ID),
    FOREIGN KEY (FLOW_ID)
        REFERENCES "FLOW"(ID)
        ON DELETE CASCADE
);

-- Table to store i18n translations
CREATE TABLE "TRANSLATION" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    MESSAGE_KEY     VARCHAR(255) NOT NULL,
    LANGUAGE_CODE   VARCHAR(10) NOT NULL,
    NAMESPACE       VARCHAR(50) NOT NULL DEFAULT 'default',
    VALUE           TEXT NOT NULL,
    CREATED_AT      TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT      TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, NAMESPACE, MESSAGE_KEY, LANGUAGE_CODE)
);

-- Index for efficient language and namespace combination lookups
CREATE INDEX idx_translation_lang_namespace ON "TRANSLATION" (DEPLOYMENT_ID, LANGUAGE_CODE);

-- Table to store OpenID4VP presentation definitions.
CREATE TABLE "PRESENTATION_DEFINITION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    HANDLE VARCHAR(255) NOT NULL,
    OU_ID VARCHAR(36) NOT NULL,
    NAME VARCHAR(255),
    DESCRIPTION VARCHAR(255),
    VCT VARCHAR(512) NOT NULL,
    FORMAT VARCHAR(64) NOT NULL DEFAULT 'dc+sd-jwt',
    CLAIMS JSONB,
    ENFORCE_TRUSTED_ISSUER BOOLEAN,
    TRUSTED_AUTHORITIES JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW()
);

-- Each presentation definition handle is unique per deployment.
CREATE UNIQUE INDEX idx_openid4vp_pd_handle ON "PRESENTATION_DEFINITION" (DEPLOYMENT_ID, HANDLE);

-- Table to store OpenID4VCI credential configurations.
CREATE TABLE "CREDENTIAL_CONFIGURATION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    HANDLE VARCHAR(255) NOT NULL,
    OU_ID VARCHAR(36) NOT NULL,
    NAME VARCHAR(255),
    DESCRIPTION VARCHAR(255),
    FORMAT VARCHAR(64) NOT NULL DEFAULT 'dc+sd-jwt',
    VCT VARCHAR(512) NOT NULL,
    CLAIMS JSONB,
    DISPLAY JSONB,
    VALIDITY_SECONDS INTEGER,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW()
);

-- Each credential configuration handle is unique per deployment.
CREATE UNIQUE INDEX idx_openid4vci_cc_handle ON "CREDENTIAL_CONFIGURATION" (DEPLOYMENT_ID, HANDLE);

-- Table to store server-wide configuration
CREATE TABLE "SERVER_CONFIG" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    NAME          VARCHAR(255) NOT NULL,
    VALUE         JSONB        NOT NULL,
    CREATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, NAME)
);

-- Table capturing the resource-sharing graph. Generic across resource types: a policy is one
-- organization unit's standing decision about one resource, and there is exactly one per
-- (resource, initiating OU) so that an edit has a single well-defined subject.
CREATE TABLE "RESOURCE_SHARING_POLICY" (
    DEPLOYMENT_ID    VARCHAR(255) NOT NULL,
    ID               VARCHAR(36) PRIMARY KEY,
    RESOURCE_TYPE    VARCHAR(64) NOT NULL,
    RESOURCE_ID      VARCHAR(36) NOT NULL,
    OWNING_OU_ID     VARCHAR(36) NOT NULL,
    INITIATING_OU_ID VARCHAR(36) NOT NULL,
    POLICY_STAGE     VARCHAR(16) NOT NULL CHECK (POLICY_STAGE IN ('share', 'reshare')),
    PARENT_POLICY_ID VARCHAR(36) REFERENCES "RESOURCE_SHARING_POLICY" (ID) ON DELETE CASCADE,
    DECLARED         BOOLEAN NOT NULL DEFAULT FALSE,
    VERSION          INTEGER NOT NULL DEFAULT 1,
    CREATED_AT       TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT       TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID, INITIATING_OU_ID)
);

CREATE INDEX idx_rsp_resource ON "RESOURCE_SHARING_POLICY" (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID);
CREATE INDEX idx_rsp_initiator ON "RESOURCE_SHARING_POLICY" (DEPLOYMENT_ID, INITIATING_OU_ID);

-- One policy names several targets, which is what splitting the target off the policy row buys.
-- The blanket scopes carry no target organization unit; the rest name exactly one.
CREATE TABLE "RESOURCE_SHARING_POLICY_TARGET" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36) PRIMARY KEY,
    POLICY_ID     VARCHAR(36) NOT NULL
                  REFERENCES "RESOURCE_SHARING_POLICY" (ID) ON DELETE CASCADE,
    TARGET_SCOPE  VARCHAR(16) NOT NULL
                  CHECK (TARGET_SCOPE IN ('all_ous', 'all_roots', 'root', 'all_children', 'ou', 'ou_subtree')),
    TARGET_OU_ID  VARCHAR(36),
    UNIQUE (POLICY_ID, TARGET_SCOPE, TARGET_OU_ID)
);

CREATE INDEX idx_rspt_policy ON "RESOURCE_SHARING_POLICY_TARGET" (POLICY_ID);
CREATE INDEX idx_rspt_target_ou ON "RESOURCE_SHARING_POLICY_TARGET" (DEPLOYMENT_ID, TARGET_OU_ID);

-- Organization units carved out of every target of a policy, each taking its subtree with it.
CREATE TABLE "RESOURCE_SHARING_POLICY_EXCLUSION" (
    DEPLOYMENT_ID  VARCHAR(255) NOT NULL,
    POLICY_ID      VARCHAR(36) NOT NULL
                   REFERENCES "RESOURCE_SHARING_POLICY" (ID) ON DELETE CASCADE,
    EXCLUDED_OU_ID VARCHAR(36) NOT NULL,
    PRIMARY KEY (POLICY_ID, EXCLUDED_OU_ID)
);

-- What a policy says a target may do with one field. TARGET_ID null means the rule applies to
-- every target; set means it overrides the policy-level rule for that target alone.
--
-- VALUE_SET and ALLOWED_SET carry the absent-versus-empty distinction into storage: zero member
-- rows is otherwise ambiguous, and omitted means unconstrained while explicitly empty permits
-- nothing. REQUESTED_EDITABLE keeps what the initiator asked for alongside what it was clamped to,
-- so re-materializing against a changed ancestor does not ratchet permanently downward.
CREATE TABLE "RESOURCE_SHARING_POLICY_OVERLAY_RULE" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    ID                  VARCHAR(36) PRIMARY KEY,
    POLICY_ID           VARCHAR(36) NOT NULL
                        REFERENCES "RESOURCE_SHARING_POLICY" (ID) ON DELETE CASCADE,
    TARGET_ID           VARCHAR(36)
                        REFERENCES "RESOURCE_SHARING_POLICY_TARGET" (ID) ON DELETE CASCADE,
    FIELD_KEY           VARCHAR(255) NOT NULL,
    EDITABLE            BOOLEAN NOT NULL,
    VALUE_SET           BOOLEAN NOT NULL DEFAULT FALSE,
    ALLOWED_SET         BOOLEAN NOT NULL DEFAULT FALSE,
    EXCLUDED_SET        BOOLEAN NOT NULL DEFAULT FALSE,
    REQUESTED_EDITABLE  BOOLEAN NOT NULL,
    REQ_VALUE_SET       BOOLEAN NOT NULL DEFAULT FALSE,
    REQ_ALLOWED_SET     BOOLEAN NOT NULL DEFAULT FALSE,
    REQ_EXCLUDED_SET    BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (POLICY_ID, TARGET_ID, FIELD_KEY)
);

CREATE INDEX idx_rspor_policy ON "RESOURCE_SHARING_POLICY_OVERLAY_RULE" (POLICY_ID);

-- One row per member, mirroring the exclusion table, so membership stays an indexed lookup rather
-- than a JSON scan. Scalars use the same table as a single value row, so the resolver has one code
-- path instead of two that can drift.
CREATE TABLE "RESOURCE_SHARING_POLICY_OVERLAY_RULE_MEMBER" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    RULE_ID       VARCHAR(36) NOT NULL
                  REFERENCES "RESOURCE_SHARING_POLICY_OVERLAY_RULE" (ID) ON DELETE CASCADE,
    MEMBER_ROLE   VARCHAR(24) NOT NULL
                  CHECK (MEMBER_ROLE IN ('value', 'allowed', 'excluded',
                                         'requestedValue', 'requestedAllowed', 'requestedExcluded')),
    MEMBER_KEY    VARCHAR(512) NOT NULL,
    PRIMARY KEY (RULE_ID, MEMBER_ROLE, MEMBER_KEY)
);

CREATE INDEX idx_rsporm_role ON "RESOURCE_SHARING_POLICY_OVERLAY_RULE_MEMBER" (RULE_ID, MEMBER_ROLE);
-- Pattern ops make hierarchical prefix matching index-assisted rather than a full scan.
CREATE INDEX idx_rsporm_prefix ON "RESOURCE_SHARING_POLICY_OVERLAY_RULE_MEMBER" (MEMBER_KEY varchar_pattern_ops);

-- A target organization unit's own value for one templated field of a shared resource.
CREATE TABLE "RESOURCE_OVERLAY_VALUE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    RESOURCE_TYPE VARCHAR(64) NOT NULL,
    RESOURCE_ID   VARCHAR(36) NOT NULL,
    OU_ID         VARCHAR(36) NOT NULL,
    FIELD_KEY     VARCHAR(255) NOT NULL,
    VALUE         JSONB NOT NULL,
    CREATED_AT    TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID, OU_ID, FIELD_KEY)
);
