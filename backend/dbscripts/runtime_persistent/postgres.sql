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
-- Part of the database.runtime_persistent classification: authoritative authorization
-- enforcement state that must survive a runtime database flush.
CREATE TABLE "REVOKED_TOKEN" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL PRIMARY KEY,
    JTI VARCHAR(255) NOT NULL,
    REVOCATION_REASON VARCHAR(30) NOT NULL CHECK (REVOCATION_REASON IN ('explicit', 'refresh_rotation')),
    REVOKED_AT TIMESTAMP NOT NULL,
    EXPIRY_TIME TIMESTAMP NOT NULL
);

-- Unique index backs the hot deny-list lookup by (deployment, jti) and enforces idempotent revocation writes.
CREATE UNIQUE INDEX idx_revoked_token_jti_deployment ON "REVOKED_TOKEN" (DEPLOYMENT_ID, JTI);

-- Index for expiry time on REVOKED_TOKEN (supports cleanup and expiry checks).
CREATE INDEX idx_revoked_token_expiry_time ON "REVOKED_TOKEN" (EXPIRY_TIME);

-- Table to store criteria-based (many-token) revocations: a generalized attribute deny list.
-- CRITERION_TYPE names the dimension ('token_family' today; subject/client/consent are future types)
-- and CRITERION_VALUE holds the revoked value (the tfid for 'token_family'). Part of the
-- database.runtime_persistent classification: authoritative enforcement state that must survive a
-- runtime database flush.
CREATE TABLE "REVOCATION_CRITERIA" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL PRIMARY KEY,
    CRITERION_TYPE VARCHAR(30) NOT NULL,
    CRITERION_VALUE VARCHAR(255) NOT NULL,
    REASON VARCHAR(30) NOT NULL,
    REVOKED_AT TIMESTAMP NOT NULL,
    EXPIRY_TIME TIMESTAMP NOT NULL
);

-- Unique index backs the hot lookup by (deployment, type, value) and enforces idempotent writes.
CREATE UNIQUE INDEX idx_revocation_criteria_lookup
    ON "REVOCATION_CRITERIA" (DEPLOYMENT_ID, CRITERION_TYPE, CRITERION_VALUE);

-- Index for expiry time on REVOCATION_CRITERIA (supports cleanup and expiry checks).
CREATE INDEX idx_revocation_criteria_expiry_time ON "REVOCATION_CRITERIA" (EXPIRY_TIME);

-- Table to store SSO sessions, grouped by flow (FLOW_ID) and resolved by an opaque handle.
-- Part of the database.runtime_persistent classification: persistent session state that must survive a
-- runtime database flush.
CREATE TABLE "SSO_SESSION" (
    SESSION_ID VARCHAR(36) NOT NULL,
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    SUBJECT_ID VARCHAR(36) NOT NULL,
    FLOW_ID VARCHAR(36) NOT NULL,
    FLOW_VERSION INTEGER NOT NULL,
    FLOW_EXECUTION_ID VARCHAR(255) NOT NULL,
    HANDLE_ID VARCHAR(255) NOT NULL,
    AUTHENTICATED_AT TIMESTAMP NOT NULL,
    CREATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    LAST_ACTIVE_AT TIMESTAMP NOT NULL,
    IDLE_EXPIRES_AT TIMESTAMP,
    ABSOLUTE_EXPIRES_AT TIMESTAMP,
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
    TFID VARCHAR(36),
    FIRST_JOINED_AT TIMESTAMP NOT NULL,
    LAST_ACTIVE_AT TIMESTAMP NOT NULL,
    PRIMARY KEY (SESSION_ID, DEPLOYMENT_ID, APP_ID)
);

-- Table to store consent records.
CREATE TABLE "CONSENT" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL PRIMARY KEY,
    GROUP_ID VARCHAR(36) NOT NULL,
    STATUS VARCHAR(20) NOT NULL,
    VALIDITY_TIME TIMESTAMPTZ,
    PURPOSES JSONB,
    CREATED_AT TIMESTAMPTZ DEFAULT NOW(),
    UPDATED_AT TIMESTAMPTZ DEFAULT NOW()
);

-- Composite index for group + status consent search.
CREATE INDEX idx_consent_group_status ON "CONSENT" (DEPLOYMENT_ID, GROUP_ID, STATUS);

-- Table to store the authorization records of a consent (1:many by CONSENT_ID).
-- USER_ID is normalized out of the consent row so consents can be searched by user.
CREATE TABLE "CONSENT_AUTHORIZATION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL PRIMARY KEY,
    CONSENT_ID VARCHAR(36) NOT NULL,
    USER_ID VARCHAR(36) NOT NULL,
    TYPE VARCHAR(20) NOT NULL,
    STATUS VARCHAR(20) NOT NULL,
    UPDATED_TIME TIMESTAMPTZ,
    FOREIGN KEY (CONSENT_ID) REFERENCES "CONSENT" (ID) ON DELETE CASCADE
);

-- Composite index for user-based consent search (join CONSENT_AUTHORIZATION -> CONSENT).
CREATE INDEX idx_consent_authz_user ON "CONSENT_AUTHORIZATION" (DEPLOYMENT_ID, USER_ID);

-- Index for loading a consent's authorization records.
CREATE INDEX idx_consent_authz_consent ON "CONSENT_AUTHORIZATION" (CONSENT_ID, DEPLOYMENT_ID);
