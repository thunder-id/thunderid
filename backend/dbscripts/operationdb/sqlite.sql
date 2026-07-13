-- ----------------------------------------------------------------------------
-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License. You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied. See the License for the
-- specific language governing permissions and limitations
-- under the License.
-- ----------------------------------------------------------------------------

-- Table to store revoked token JTIs (single-token revocation deny list).
-- Part of the database.operation classification: authoritative authorization
-- enforcement state that must survive a runtime database flush.
CREATE TABLE "REVOKED_TOKEN" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL PRIMARY KEY,
    JTI VARCHAR(255) NOT NULL,
    REVOCATION_REASON VARCHAR(30) NOT NULL CHECK (REVOCATION_REASON IN ('explicit', 'refresh_rotation')),
    REVOKED_AT DATETIME NOT NULL,
    EXPIRY_TIME DATETIME NOT NULL
);

-- Unique index backs the hot deny-list lookup by (deployment, jti) and enforces idempotent revocation writes.
CREATE UNIQUE INDEX idx_revoked_token_jti_deployment ON "REVOKED_TOKEN" (DEPLOYMENT_ID, JTI);

-- Index for expiry time on REVOKED_TOKEN (supports cleanup and expiry checks).
CREATE INDEX idx_revoked_token_expiry_time ON "REVOKED_TOKEN" (EXPIRY_TIME);

-- Table to store SSO sessions, grouped by flow (FLOW_ID) and resolved by an opaque handle.
-- Part of the database.operation classification: persistent session state that must survive a
-- runtime database flush.
CREATE TABLE "SSO_SESSION" (
    SESSION_ID VARCHAR(36) NOT NULL,
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    SUBJECT_ID VARCHAR(36) NOT NULL,
    FLOW_ID VARCHAR(36) NOT NULL,
    FLOW_VERSION INTEGER NOT NULL,
    FLOW_EXECUTION_ID VARCHAR(255) NOT NULL,
    HANDLE_ID VARCHAR(255) NOT NULL,
    AUTHENTICATED_AT DATETIME NOT NULL,
    CREATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    LAST_ACTIVE_AT DATETIME NOT NULL,
    IDLE_EXPIRES_AT DATETIME,
    ABSOLUTE_EXPIRES_AT DATETIME,
    STATE VARCHAR(50) NOT NULL,
    VERSION INTEGER NOT NULL,
    UPDATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (SESSION_ID, DEPLOYMENT_ID)
);

-- Unique index for handle lookup on SSO_SESSION (one session per handle, per deployment)
CREATE UNIQUE INDEX idx_sso_session_handle_id ON "SSO_SESSION" (HANDLE_ID, DEPLOYMENT_ID);

-- Unique index enforcing one session per establishing flow execution (per deployment). Lets
-- concurrent joins in a single flow execution converge on one session instead of duplicating it.
CREATE UNIQUE INDEX idx_sso_session_flow_execution ON "SSO_SESSION" (FLOW_EXECUTION_ID, DEPLOYMENT_ID);

-- Index for subject + flow lookup on SSO_SESSION
CREATE INDEX idx_sso_session_subject_flow ON "SSO_SESSION" (SUBJECT_ID, FLOW_ID, DEPLOYMENT_ID);

-- Index for absolute expiry on SSO_SESSION (supports cleanup)
CREATE INDEX idx_sso_session_absolute_expires_at ON "SSO_SESSION" (ABSOLUTE_EXPIRES_AT);

-- Table to store the durable session context for an SSO session, one row per checkpoint.
CREATE TABLE "SSO_SESSION_CONTEXT" (
    SESSION_ID VARCHAR(36) NOT NULL,
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    CHECKPOINT_ID VARCHAR(255) NOT NULL,
    CONTEXT TEXT,
    CONTEXT_VERSION INTEGER NOT NULL,
    CREATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (SESSION_ID, DEPLOYMENT_ID, CHECKPOINT_ID)
);

-- Table to record the applications participating in an SSO session (1:many by SESSION_ID).
CREATE TABLE "SSO_SESSION_PARTICIPANT" (
    SESSION_ID VARCHAR(36) NOT NULL,
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    APP_ID VARCHAR(36) NOT NULL,
    FIRST_JOINED_AT DATETIME NOT NULL,
    LAST_ACTIVE_AT DATETIME NOT NULL,
    PRIMARY KEY (SESSION_ID, DEPLOYMENT_ID, APP_ID)
);
