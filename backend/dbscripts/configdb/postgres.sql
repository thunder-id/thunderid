-- Table to store Entity Schemas (user/agent categories)
CREATE TABLE "ENTITY_TYPES" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    ID          VARCHAR(36) NOT NULL,
    CATEGORY    VARCHAR(50) NOT NULL,
    NAME        VARCHAR(100) NOT NULL,
    HANDLE      VARCHAR(100) NOT NULL,
    OU_ID       VARCHAR(36) NOT NULL,
    ALLOW_SELF_REGISTRATION BOOLEAN DEFAULT FALSE NOT NULL,
    SCHEMA_DEF  JSONB NOT NULL,
    SYSTEM_ATTRIBUTES JSONB,
    CREATED_AT  TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (HANDLE, CATEGORY, DEPLOYMENT_ID),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Composite index for deployment + category + OU-based entity type lookups
CREATE INDEX idx_entity_schemas_deployment_category_ou ON "ENTITY_TYPES" (DEPLOYMENT_ID, CATEGORY, OU_ID);

-- Table to store Roles
CREATE TABLE "ROLE" (
    DEPLOYMENT_ID           VARCHAR(255) NOT NULL,
    ID                  VARCHAR(36) NOT NULL,
    OU_ID               VARCHAR(36) NOT NULL,
    NAME                VARCHAR(50) NOT NULL,
    DESCRIPTION         VARCHAR(255),
    CREATED_AT          TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT          TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT unique_role_ou_name UNIQUE (OU_ID, NAME, DEPLOYMENT_ID),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
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
    FOREIGN KEY (DEPLOYMENT_ID, ROLE_ID)
        REFERENCES "ROLE" (DEPLOYMENT_ID, ID) ON DELETE CASCADE
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
    ID VARCHAR(36) NOT NULL,
    DISPLAY_NAME VARCHAR(255) NOT NULL,
    HANDLE VARCHAR(255) NOT NULL,
    DESCRIPTION VARCHAR(512),
    THEME JSONB NOT NULL,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (DEPLOYMENT_ID, HANDLE),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Index for deployment isolation on THEME
CREATE INDEX idx_theme_deployment_id ON "THEME" (DEPLOYMENT_ID);

-- Unique index for theme handle per deployment
CREATE UNIQUE INDEX idx_theme_handle_deployment ON "THEME" (HANDLE, DEPLOYMENT_ID);

-- Table to store layout configurations.
CREATE TABLE "LAYOUT" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    DISPLAY_NAME VARCHAR(255) NOT NULL,
    HANDLE VARCHAR(255) NOT NULL,
    DESCRIPTION VARCHAR(512),
    LAYOUT JSONB NOT NULL,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (DEPLOYMENT_ID, HANDLE),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Index for deployment isolation on LAYOUT
CREATE INDEX idx_layout_deployment_id ON "LAYOUT" (DEPLOYMENT_ID);

-- Unique index for layout handle per deployment
CREATE UNIQUE INDEX idx_layout_handle_deployment ON "LAYOUT" (HANDLE, DEPLOYMENT_ID);

-- Table to store inbound client configurations for an entity.
CREATE TABLE "INBOUND_CLIENT" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ENTITY_ID VARCHAR(36) NOT NULL,
    AUTH_FLOW_ID VARCHAR(100) NOT NULL,
    REGISTRATION_FLOW_ID VARCHAR(100),
    IS_REGISTRATION_FLOW_ENABLED CHAR(1) DEFAULT '1',
    RECOVERY_FLOW_ID VARCHAR(100),
    IS_RECOVERY_FLOW_ENABLED CHAR(1) DEFAULT '0',
    SIGNOUT_FLOW_ID VARCHAR(100),
    THEME_ID VARCHAR(36),
    LAYOUT_ID VARCHAR(36),
    PROPERTIES JSONB,
    PRIMARY KEY (DEPLOYMENT_ID, ENTITY_ID)
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
    FOREIGN KEY (DEPLOYMENT_ID, ENTITY_ID)
        REFERENCES "INBOUND_CLIENT" (DEPLOYMENT_ID, ENTITY_ID) ON DELETE CASCADE
);

-- Table to store identity providers.
CREATE TABLE "IDP" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    NAME VARCHAR(255) NOT NULL,
    DESCRIPTION VARCHAR(500),
    TYPE VARCHAR(20) NOT NULL,
    OU_ID VARCHAR(36) NOT NULL,
    PROPERTIES JSONB,
    ATTRIBUTE_CONFIGURATION JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Composite index for name-based IDP lookups
CREATE INDEX idx_idp_name_deployment ON "IDP" (DEPLOYMENT_ID, NAME);

-- Composite index for deployment + owning organization unit lookups.
CREATE INDEX idx_idp_ou_deployment ON "IDP" (DEPLOYMENT_ID, OU_ID);

-- Expression index for issuer-based IDP lookups
CREATE INDEX idx_idp_issuer ON "IDP" (DEPLOYMENT_ID, (PROPERTIES->'issuer'->>'value'));

-- Table to store notification senders.
CREATE TABLE "NOTIFICATION_SENDER" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    NAME VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    DESCRIPTION VARCHAR(500),
    TYPE VARCHAR(20) NOT NULL,
    PROVIDER VARCHAR(20) NOT NULL,
    PROPERTIES JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Composite index for name-based notification sender lookups
CREATE INDEX idx_notification_sender_name_deployment ON "NOTIFICATION_SENDER" (DEPLOYMENT_ID, NAME);

-- Table to store external authorization PDP connections.
CREATE TABLE "AUTHORIZATION_PDP_CONNECTION" (
	DEPLOYMENT_ID VARCHAR(255) NOT NULL,
	ID VARCHAR(36) PRIMARY KEY,
	NAME VARCHAR(255) NOT NULL,
	DESCRIPTION VARCHAR(500),
	TYPE VARCHAR(50) NOT NULL,
	PROPERTIES JSONB NOT NULL,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW()
);

-- Unique constraint: Authorization PDP provider names must be unique per provider type and deployment.
CREATE UNIQUE INDEX idx_authorization_pdp_connection_type_name_deployment
	ON "AUTHORIZATION_PDP_CONNECTION" (DEPLOYMENT_ID, TYPE, NAME);

-- Table to store outbound HTTP endpoint connections. Secret values in PROPERTIES are encrypted.
CREATE TABLE "HTTP_CONNECTION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36) NOT NULL,
    NAME          VARCHAR(255) NOT NULL,
    DESCRIPTION   VARCHAR(500),
    PROPERTIES    JSONB NOT NULL,
    CREATED_AT    TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Unique constraint: HTTP connection names must be unique per deployment.
CREATE UNIQUE INDEX idx_http_connection_name_deployment
    ON "HTTP_CONNECTION" (DEPLOYMENT_ID, NAME);

-- Table to store certificates associated with various entities.
CREATE TABLE "CERTIFICATE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    REF_TYPE VARCHAR(20) NOT NULL,
    REF_ID VARCHAR(2048) NOT NULL,
    TYPE VARCHAR(20) NOT NULL,
    VALUE TEXT NOT NULL,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (REF_TYPE, REF_ID, DEPLOYMENT_ID),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Table to store resource servers.
CREATE TABLE "RESOURCE_SERVER" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    OU_ID VARCHAR(36) NOT NULL,
    NAME VARCHAR(100) NOT NULL,
    DESCRIPTION TEXT,
    IDENTIFIER VARCHAR(2048) NOT NULL,
    TYPE VARCHAR(20) CHECK (TYPE IS NULL OR TYPE IN ('API', 'MCP', 'CUSTOM')),
    PROPERTIES JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (OU_ID, NAME, DEPLOYMENT_ID),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Composite index for name-based resource server lookups
CREATE INDEX idx_resource_server_name_deployment ON "RESOURCE_SERVER" (DEPLOYMENT_ID, NAME);

-- Unique constraint: Resource server identifier must be unique per deployment
CREATE UNIQUE INDEX uq_resource_server_identifier
    ON "RESOURCE_SERVER"(IDENTIFIER, DEPLOYMENT_ID);

-- Table to store resources within resource servers.
CREATE TABLE "RESOURCE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    RESOURCE_SERVER_ID VARCHAR(36) NOT NULL,
    PARENT_RESOURCE_ID VARCHAR(36),
    NAME VARCHAR(100) NOT NULL,
    HANDLE VARCHAR(100) NOT NULL,
    DESCRIPTION TEXT,
    PERMISSION VARCHAR(1000) NOT NULL,
    PROPERTIES JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),

    FOREIGN KEY (DEPLOYMENT_ID, RESOURCE_SERVER_ID)
        REFERENCES "RESOURCE_SERVER" (DEPLOYMENT_ID, ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    FOREIGN KEY (DEPLOYMENT_ID, PARENT_RESOURCE_ID)
        REFERENCES "RESOURCE" (DEPLOYMENT_ID, ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    PRIMARY KEY (DEPLOYMENT_ID, ID)
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
    ID VARCHAR(36) NOT NULL,
    RESOURCE_SERVER_ID VARCHAR(36) NOT NULL,
    RESOURCE_ID VARCHAR(36),
    NAME VARCHAR(100) NOT NULL,
    HANDLE VARCHAR(100) NOT NULL,
    DESCRIPTION TEXT,
    PERMISSION VARCHAR(1000) NOT NULL,
    PROPERTIES JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),

    FOREIGN KEY (DEPLOYMENT_ID, RESOURCE_SERVER_ID)
        REFERENCES "RESOURCE_SERVER" (DEPLOYMENT_ID, ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    FOREIGN KEY (DEPLOYMENT_ID, RESOURCE_ID)
        REFERENCES "RESOURCE" (DEPLOYMENT_ID, ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    PRIMARY KEY (DEPLOYMENT_ID, ID)
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
    ID VARCHAR(36) NOT NULL,
    HANDLE VARCHAR(100) NOT NULL,
    NAME VARCHAR(100) NOT NULL,
    FLOW_TYPE VARCHAR(50) NOT NULL,
    ACTIVE_VERSION INTEGER NOT NULL,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (HANDLE, FLOW_TYPE, DEPLOYMENT_ID),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
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
    FOREIGN KEY (DEPLOYMENT_ID, FLOW_ID)
        REFERENCES "FLOW" (DEPLOYMENT_ID, ID)
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
    ID VARCHAR(36) NOT NULL,
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
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Each presentation definition handle is unique per deployment.
CREATE UNIQUE INDEX idx_openid4vp_pd_handle ON "PRESENTATION_DEFINITION" (DEPLOYMENT_ID, HANDLE);

-- Table to store OpenID4VCI credential configurations.
CREATE TABLE "CREDENTIAL_CONFIGURATION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
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
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
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

-- Named values this deployment holds, for configuration to refer to instead of carrying inline.
--
-- Two tables rather than one with a flag, because the two differ in what may be read back: a
-- variable's value is returned by the API and a secret's never is. Keeping them apart means a query
-- written against variables cannot reach a secret by forgetting a predicate.
CREATE TABLE "VARIABLE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    NAME          VARCHAR(255) NOT NULL,
    VALUE         TEXT         NOT NULL,
    DESCRIPTION   TEXT,
    CREATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, NAME)
);
-- Listing is always scoped to a deployment and ordered by name, which the primary key already
-- serves; this covers the prefix match a filter does.
CREATE INDEX idx_variable_deployment_name ON "VARIABLE" (DEPLOYMENT_ID, NAME);

-- A secret's value is stored encrypted, never in the clear. VALUE therefore holds ciphertext and is
-- meaningless without the deployment's configuration key.
CREATE TABLE "SECRET" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    NAME          VARCHAR(255) NOT NULL,
    VALUE         TEXT         NOT NULL,
    DESCRIPTION   TEXT,
    CREATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, NAME)
);
CREATE INDEX idx_secret_deployment_name ON "SECRET" (DEPLOYMENT_ID, NAME);

-- Table to store the gateways this control plane administers. A deployment holds at most
-- server.max_gateways of them, one by default.
CREATE TABLE "GATEWAY" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    NAME VARCHAR(255) NOT NULL,
    BASE_URL TEXT NOT NULL,
    MANAGEMENT_KEY TEXT NOT NULL,
    CA_CERTIFICATE TEXT,
    -- The default gateway, whose URL the console shows for an application's runtime endpoints. At
    -- most one per deployment, which the index below enforces.
    IS_DEFAULT BOOLEAN NOT NULL DEFAULT FALSE,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (NAME, DEPLOYMENT_ID),
    -- One gateway registers once, and its address is what says which one it is.
    UNIQUE (BASE_URL, DEPLOYMENT_ID)
);

-- At most one gateway of a deployment is the default. A partial index rather than a table
-- constraint, because the rule is about the rows that are the default, not about the column.
CREATE UNIQUE INDEX idx_gateway_default_deployment
    ON "GATEWAY" (DEPLOYMENT_ID)
    WHERE IS_DEFAULT = TRUE;

-- A captured state of this deployment's configuration, named by a hash of what it captured and
-- ordered per deployment. It is what is applied to a gateway. Its values, when the export carried
-- any, are held encrypted as one secret. CREATED_BY is the id of the user it was
-- captured for; their name is read from the trusted issuer when shown, never stored.
CREATE TABLE "CONFIGURATION_VERSION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) PRIMARY KEY,
    SEQ INTEGER NOT NULL,
    HASH VARCHAR(64) NOT NULL,
    NAME VARCHAR(255),
    RESOURCES TEXT NOT NULL,
    VARIABLES TEXT,
    NOTE TEXT,
    IS_CURRENT CHAR(1) NOT NULL DEFAULT '0',
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    CREATED_BY VARCHAR(255),
    UNIQUE (DEPLOYMENT_ID, SEQ),
    UNIQUE (DEPLOYMENT_ID, HASH)
);

-- The configuration version each gateway holds, and the one it held before, which is what a revert
-- returns it to, with who applied it. A gateway declared in a file has no GATEWAY row, so there is no foreign key.
CREATE TABLE "GATEWAY_APPLIED_VERSION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    GATEWAY_ID VARCHAR(36) NOT NULL,
    APPLIED_SEQ INTEGER NOT NULL,
    PREVIOUS_SEQ INTEGER,
    APPLIED_AT TIMESTAMPTZ DEFAULT NOW(),
    APPLIED_BY VARCHAR(255),
    -- When each value the applied version refers to was last set, as the gateway's store said at the
    -- apply, so the next apply reports those set again since by comparing the store's own times.
    HELD_VALUES TEXT,
    PRIMARY KEY (GATEWAY_ID, DEPLOYMENT_ID)
);

-- One apply or revert at a time per gateway, across every node of this plane: an apply claims the
-- row with a token of its own before it sends anything and clears it when done, and a claim older
-- than an import may take is taken over. PENDING_LATEST records that the latest version is to be
-- applied again once the running apply finishes, for whoever asked while it ran.
CREATE TABLE "GATEWAY_APPLY_LEASE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    GATEWAY_ID VARCHAR(36) NOT NULL,
    TOKEN VARCHAR(36),
    APPLYING_SINCE TIMESTAMPTZ,
    PENDING_LATEST BOOLEAN NOT NULL DEFAULT FALSE,
    PENDING_BY VARCHAR(255),
    PRIMARY KEY (DEPLOYMENT_ID, GATEWAY_ID)
);

-- The resources each gateway is set to leave alone: an apply neither writes nor removes them there,
-- until one selects them again. A resource is keyed by its type and its id, or its name when it has
-- none. Its type, id and name are kept too, so one whose removal was held back can still be offered
-- once no version names it. A gateway declared in a file has no GATEWAY row, so there is no foreign
-- key.
CREATE TABLE "GATEWAY_EXCLUDED_RESOURCE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    GATEWAY_ID VARCHAR(36) NOT NULL,
    RESOURCE_KEY VARCHAR(512) NOT NULL,
    RESOURCE_TYPE VARCHAR(255) NOT NULL,
    RESOURCE_ID VARCHAR(255),
    RESOURCE_NAME VARCHAR(255),
    -- A resource skipped for one apply is owed to the gateway until an apply writes or removes it,
    -- rather than left alone for good.
    ONCE BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (DEPLOYMENT_ID, GATEWAY_ID, RESOURCE_KEY)
);

-- Table capturing the resource-sharing graph. Generic across resource types: a policy is one
-- organization unit's standing decision about one resource, and there is exactly one per
-- (resource, initiating OU) so that an edit has a single well-defined subject.
--
-- SECTIONS says which sections the policy states, in which mode, and what each one excludes, as one
-- document keyed by section: {"childUnits": {"mode": "all", "excludedOuIds": ["<ou id>"]}}. The
-- organization units a section names one by one are rows of RESOURCE_SHARING_POLICY_UNIT.
CREATE TABLE "RESOURCE_SHARING_POLICY" (
    DEPLOYMENT_ID    VARCHAR(255) NOT NULL,
    ID               VARCHAR(36) NOT NULL,
    RESOURCE_TYPE    VARCHAR(64) NOT NULL,
    RESOURCE_ID      VARCHAR(36) NOT NULL,
    OWNING_OU_ID     VARCHAR(36) NOT NULL,
    INITIATING_OU_ID VARCHAR(36) NOT NULL,
    POLICY_STAGE     VARCHAR(16) NOT NULL CHECK (POLICY_STAGE IN ('share', 'reshare')),
    PARENT_POLICY_ID VARCHAR(36),
    SECTIONS         JSONB NOT NULL,
    VERSION          INTEGER NOT NULL DEFAULT 1,
    CREATED_AT       TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT       TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    UNIQUE (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID, INITIATING_OU_ID)
);

CREATE INDEX idx_rsp_parent ON "RESOURCE_SHARING_POLICY" (DEPLOYMENT_ID, PARENT_POLICY_ID);

-- The organization units a policy names one by one: the direct children of a 'selected' child-units
-- decision, and the roots of a 'selected' other-tree-roots decision. A blanket decision names none,
-- so it has no rows here. A unit is named once per policy, and only a child unit may carry the
-- resource on beneath itself.
CREATE TABLE "RESOURCE_SHARING_POLICY_UNIT" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    POLICY_ID     VARCHAR(36) NOT NULL,
    SECTION       VARCHAR(16) NOT NULL CHECK (SECTION IN ('childUnits', 'otherTreeRoots')),
    OU_ID         VARCHAR(36) NOT NULL,
    REACH         VARCHAR(16) NOT NULL CHECK (REACH IN ('unit', 'subtree')),
    PRIMARY KEY (DEPLOYMENT_ID, POLICY_ID, OU_ID),
    FOREIGN KEY (DEPLOYMENT_ID, POLICY_ID)
        REFERENCES "RESOURCE_SHARING_POLICY" (DEPLOYMENT_ID, ID) ON DELETE CASCADE,
    CHECK (SECTION = 'childUnits' OR REACH = 'unit')
);

CREATE INDEX idx_rspu_ou ON "RESOURCE_SHARING_POLICY_UNIT" (DEPLOYMENT_ID, OU_ID);

-- What a policy says the units it reaches may do with one field. A blanket decision carries one set
-- of rules for everything it reaches, stored with a null OU_ID; a unit named one by one carries its
-- own, stored against that unit.
CREATE TABLE "RESOURCE_SHARING_POLICY_OVERLAY_RULE" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    ID                  VARCHAR(36) NOT NULL,
    POLICY_ID           VARCHAR(36) NOT NULL,
    SECTION             VARCHAR(16) NOT NULL CHECK (SECTION IN ('allUnits', 'childUnits', 'otherTreeRoots')),
    OU_ID               VARCHAR(36),
    FIELD_KEY           VARCHAR(255) NOT NULL,
    RESOLVED            JSONB NOT NULL,
    REQUESTED           JSONB NOT NULL,
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    FOREIGN KEY (DEPLOYMENT_ID, POLICY_ID)
        REFERENCES "RESOURCE_SHARING_POLICY" (DEPLOYMENT_ID, ID) ON DELETE CASCADE,
    UNIQUE (DEPLOYMENT_ID, POLICY_ID, SECTION, OU_ID, FIELD_KEY)
);

CREATE INDEX idx_rspor_policy ON "RESOURCE_SHARING_POLICY_OVERLAY_RULE" (DEPLOYMENT_ID, POLICY_ID);

-- The UNIQUE above leaves a blanket decision's rules unconstrained: their OU_ID is NULL, and both
-- engines count NULLs as distinct, so one decision could hold the same field twice.
CREATE UNIQUE INDEX idx_rspor_blanket_once
    ON "RESOURCE_SHARING_POLICY_OVERLAY_RULE" (DEPLOYMENT_ID, POLICY_ID, SECTION, FIELD_KEY)
    WHERE OU_ID IS NULL;

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

-- Table to store notification templates. Content is a single JSON document so a new channel can
-- introduce its own content fields without a schema migration; the per-channel shape is validated at
-- the service layer. HANDLE is the immutable, unique reference for a template within a channel and is
-- unique per channel within a deployment; DISPLAY_NAME is the human-readable label.
CREATE TABLE "NOTIFICATION_TEMPLATE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  NOT NULL,
    CHANNEL       VARCHAR(16)  NOT NULL,
    HANDLE        VARCHAR(255) NOT NULL,
    OU_ID         VARCHAR(36)  NOT NULL,
    DISPLAY_NAME  VARCHAR(255) NOT NULL,
    DESCRIPTION   VARCHAR(512),
    CONTENT       JSONB        NOT NULL,
    DESIGN        JSONB        NOT NULL DEFAULT '{}',
    CREATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    UNIQUE (DEPLOYMENT_ID, CHANNEL, HANDLE)
);
