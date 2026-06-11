-- Table to store OAuth2 authorization codes.
CREATE TABLE "AUTHORIZATION_CODE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    CODE_ID VARCHAR(36) PRIMARY KEY,
    AUTHORIZATION_CODE VARCHAR(500) NOT NULL,
    CLIENT_ID VARCHAR(255) NOT NULL,
    STATE VARCHAR(50) NOT NULL,
    AUTHZ_DATA JSONB NOT NULL,
    TIME_CREATED TIMESTAMP NOT NULL,
    EXPIRY_TIME TIMESTAMP NOT NULL
);

-- Composite index for authorization code lookup by code + deployment (hot login-path query)
CREATE INDEX idx_authorization_code_code_deployment ON "AUTHORIZATION_CODE" (AUTHORIZATION_CODE, DEPLOYMENT_ID);

-- Index for expiry time on AUTHORIZATION_CODE (supports cleanup and expiry checks)
CREATE INDEX idx_authz_code_expiry_time ON "AUTHORIZATION_CODE" (EXPIRY_TIME);

-- Table to store OAuth2 authorization request context
CREATE TABLE "AUTHORIZATION_REQUEST" (
    AUTH_ID VARCHAR(36) NOT NULL,
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    REQUEST_DATA JSONB NOT NULL,
    EXPIRY_TIME TIMESTAMP NOT NULL,
    CREATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (AUTH_ID, DEPLOYMENT_ID)
);

-- Index for expiry time on AUTHORIZATION_REQUEST (supports cleanup and expiry checks)
CREATE INDEX idx_authorization_request_expiry_time ON "AUTHORIZATION_REQUEST" (EXPIRY_TIME);

-- Table to store OAuth2 CIBA (Client-Initiated Backchannel Authentication) requests.
-- USER_ID is NULL at creation and populated at callback once the user authenticates.
-- EXECUTION_ID is intentionally omitted: it is transient, lives only in the notification
-- link URL and the FLOW_CONTEXT table, and is never needed for polling or token issuance.
CREATE TABLE "CIBA_AUTH_REQUEST" (
    AUTH_REQ_ID        VARCHAR(36)  NOT NULL,
    DEPLOYMENT_ID      VARCHAR(255) NOT NULL,
    CLIENT_ID          VARCHAR(255) NOT NULL,
    USER_ID            VARCHAR(36),
    STANDARD_SCOPES    TEXT         NOT NULL,
    STATE              VARCHAR(50)  NOT NULL,
    AUTHORIZED_SCOPES  TEXT,
    ATTRIBUTE_CACHE_ID VARCHAR(36),
    COMPLETED_ACR      VARCHAR(255),
    AUTH_TIME          TIMESTAMP,
    LAST_POLLED_AT     TIMESTAMP,
    EXPIRY_TIME        TIMESTAMP    NOT NULL,
    CREATED_AT         TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (AUTH_REQ_ID, DEPLOYMENT_ID)
);

-- Index for expiry time on CIBA_AUTH_REQUEST (supports cleanup and expiry checks)
CREATE INDEX idx_ciba_auth_request_expiry_time ON "CIBA_AUTH_REQUEST" (EXPIRY_TIME);

-- Table to store flow context
CREATE TABLE "FLOW_CONTEXT" (
    FLOW_ID VARCHAR(36) NOT NULL,
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    CONTEXT JSONB,
    EXPIRY_TIME TIMESTAMP NOT NULL,
    CREATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UPDATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (FLOW_ID, DEPLOYMENT_ID)
);

-- Index for deployment isolation on FLOW_CONTEXT
CREATE INDEX idx_flow_context_deployment_id ON "FLOW_CONTEXT" (DEPLOYMENT_ID);

-- Index for expiry time on FLOW_CONTEXT
CREATE INDEX idx_flow_context_expiry_time ON "FLOW_CONTEXT" (EXPIRY_TIME);

-- Table to store WebAuthn session data
CREATE TABLE "WEBAUTHN_SESSION" (
    SESSION_KEY VARCHAR(255) NOT NULL,
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    SESSION_DATA JSONB NOT NULL,
    CREATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    EXPIRY_TIME TIMESTAMP NOT NULL,
    PRIMARY KEY (SESSION_KEY, DEPLOYMENT_ID)
);

-- Index for expiry time on WEBAUTHN_SESSION
CREATE INDEX idx_webauthn_session_expiry_time ON "WEBAUTHN_SESSION" (EXPIRY_TIME);

-- Table to store attribute cache entries
CREATE TABLE "ATTRIBUTE_CACHE" (
    ID VARCHAR(36) PRIMARY KEY,
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ATTRIBUTES JSONB NOT NULL,
    EXPIRY_TIME TIMESTAMP NOT NULL,
    CREATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Table to store pushed authorization requests (PAR)
CREATE TABLE "PAR_REQUEST" (
    REQUEST_URI VARCHAR(43) PRIMARY KEY,
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    REQUEST_PARAMS JSONB NOT NULL,
    EXPIRY_TIME TIMESTAMP NOT NULL
);

-- Index for expiry time on PAR_REQUEST (supports cleanup and expiry checks)
CREATE INDEX idx_par_request_expiry_time ON "PAR_REQUEST" (EXPIRY_TIME);

-- Table to store JWT jti values for replay protection across consumers. Rows are isolated by NAMESPACE.
CREATE TABLE "JTI_RECORD" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    NAMESPACE VARCHAR(64) NOT NULL,
    JTI VARCHAR(256) NOT NULL,
    EXPIRY_TIME TIMESTAMP NOT NULL,
    PRIMARY KEY (DEPLOYMENT_ID, NAMESPACE, JTI)
);

-- Index for expiry time on JTI_RECORD (supports cleanup and expiry checks)
CREATE INDEX idx_jti_record_expiry_time ON "JTI_RECORD" (EXPIRY_TIME);
