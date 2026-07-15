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

-- Token Status List (draft-ietf-oauth-status-list): one row per status list, holding the monotonic
-- index allocator counter and lifecycle state. Part of the database.operation classification:
-- authoritative revocation state that must survive a runtime database flush. ID is an opaque UUID used
-- as the suffix of the list's public URI (no identity encoded, for herd privacy).
CREATE TABLE "STATUS_LIST" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    BITS SMALLINT NOT NULL DEFAULT 1,
    STATE SMALLINT NOT NULL DEFAULT 0,
    NEXT_IDX BIGINT NOT NULL DEFAULT 0,
    CAPACITY BIGINT NOT NULL,
    CREATED_AT DATETIME NOT NULL,
    SEALED_AT DATETIME,
    PRIMARY KEY (ID)
);

-- Index to locate the active (unsealed) list for a deployment during index allocation.
CREATE INDEX idx_status_list_deployment_state ON "STATUS_LIST" (DEPLOYMENT_ID, STATE);

-- Index on seal time to find sealed lists whose retention has elapsed (bulk drop).
CREATE INDEX idx_status_list_sealed_at ON "STATUS_LIST" (SEALED_AT);

-- Sparse status entries: one row ONLY per non-VALID (revoked) referenced token. VALID tokens write
-- nothing, so the table size tracks revocations, not issuance. IDX is the token's allocated slot; the
-- primary key doubles as the enforcement point-lookup and the publish scan (by DEPLOYMENT_ID, LIST_ID).
CREATE TABLE "STATUS_LIST_ENTRY" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    LIST_ID VARCHAR(36) NOT NULL,
    IDX BIGINT NOT NULL,
    STATUS SMALLINT NOT NULL DEFAULT 1,
    EXPIRY_TIME DATETIME NOT NULL,
    UPDATED_AT DATETIME NOT NULL,
    PRIMARY KEY (DEPLOYMENT_ID, LIST_ID, IDX),
    FOREIGN KEY (LIST_ID) REFERENCES "STATUS_LIST" (ID) ON DELETE CASCADE
);

-- Index on expiry time supports secondary reaping of entries whose tokens have already expired.
CREATE INDEX idx_status_list_entry_expiry_time ON "STATUS_LIST_ENTRY" (EXPIRY_TIME);

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
