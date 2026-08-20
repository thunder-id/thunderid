-- ----------------------------------------------------------------------------
-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
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

-- The environment manager's own datasource. Only a Control Plane has one: no other plane promotes
-- configuration, so no other plane runs an environment manager.
--
-- DEPLOYMENT_ID here is the organization, not one of its environments. A deployment id names an
-- environment as "<org>:<env>", and promotion compares one environment against another, so the whole
-- chain an organization promotes through has to sit in one partition.

-- Environments configuration is promoted through, one row per environment.
--
-- DATA is the environment document. Nothing queries inside it: an organization has a handful of
-- environments, they are read as a set, and ordering and rank are resolved in the server.
CREATE TABLE "ENVIRONMENT" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  NOT NULL,
    DATA          TEXT         NOT NULL,
    CREATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Configuration versions captured from an environment, which promotion compares and applies.
--
-- SEQ is assigned per environment and rises by one, so (DEPLOYMENT_ID, ENV_ID, SEQ) both identifies
-- a version and orders the history. Deleting an environment takes its versions with it.
CREATE TABLE "ENVIRONMENT_VERSION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ENV_ID        VARCHAR(36)  NOT NULL,
    SEQ           INTEGER      NOT NULL,
    DATA          TEXT         NOT NULL,
    CREATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ENV_ID, SEQ),
    CONSTRAINT fk_environment_version_environment
        FOREIGN KEY (DEPLOYMENT_ID, ENV_ID) REFERENCES "ENVIRONMENT" (DEPLOYMENT_ID, ID)
        ON DELETE CASCADE
);

-- Non-secret environment variables, held per environment. KEY is the declarative placeholder the
-- value resolves (e.g. MY_APP_REDIRECT_URL); VALUE is stored in plaintext because it carries no
-- confidential material.
--
-- A variable belongs to one environment of the organization, because its value is a property of the
-- deployment it is applied to: a redirect URL differs between dev and prod even though the
-- configuration referring to it is the same. Deleting an environment takes its variables with it.
CREATE TABLE "ENVIRONMENT_VARIABLE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  PRIMARY KEY,
    ENV_ID        VARCHAR(36)  NOT NULL,
    KEY           VARCHAR(255) NOT NULL,
    VALUE         TEXT         NOT NULL,
    DESCRIPTION   VARCHAR(255),
    CREATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    CONSTRAINT unique_environment_variable_key UNIQUE (DEPLOYMENT_ID, ENV_ID, KEY),
    CONSTRAINT fk_environment_variable_environment
        FOREIGN KEY (DEPLOYMENT_ID, ENV_ID) REFERENCES "ENVIRONMENT" (DEPLOYMENT_ID, ID)
        ON DELETE CASCADE
);

CREATE INDEX idx_environment_variable_environment ON "ENVIRONMENT_VARIABLE" (DEPLOYMENT_ID, ENV_ID);

-- Work queued for a Data Plane, and what it answered.
--
-- A Control Plane pod can only speak to the Data Planes that dialled it, so a request arriving at a
-- pod holding no link would otherwise fail. Every request is written here first and then delivered:
-- by this pod when it holds a link, or by whichever pod does. The caller is handed the id and reads
-- the answer back from this table, from any pod.
--
-- Rows are never deleted here. Pruning is a separate concern.
CREATE TABLE "DATA_PLANE_JOB" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(64)  NOT NULL,
    -- The deployment this is for, as "<org>:<env>". DEPLOYMENT_ID above is the organization, so that
    -- an organization's queue sits in one partition with its environments.
    DATA_PLANE_ID VARCHAR(255) NOT NULL,
    ENV_ID        VARCHAR(64),
    -- What to do: "import" applies configuration, "secret_put" stores one credential.
    TYPE          VARCHAR(32)  NOT NULL,
    -- The request, as JSON. Encrypted when it carries a credential, which is what ENCRYPTED records:
    -- a secret is held here only until it is delivered, and never in the clear.
    PAYLOAD       TEXT         NOT NULL,
    ENCRYPTED     CHAR(1)      DEFAULT '0' NOT NULL,
    -- pending -> claimed -> done | failed.
    STATUS        VARCHAR(16)  NOT NULL,
    -- Which pod is delivering it, for diagnosing a job stuck in claimed.
    CLAIMED_BY    VARCHAR(255),
    -- What the Data Plane answered, as JSON, or why it could not be delivered.
    RESULT        TEXT,
    ERROR         TEXT,
    ATTEMPTS      INTEGER      DEFAULT 0 NOT NULL,
    CREATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    COMPLETED_AT  TIMESTAMPTZ,
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Claiming reads the oldest pending row for one Data Plane, and checks whether that Data Plane
-- already has one in flight.
CREATE INDEX idx_data_plane_job_queue ON "DATA_PLANE_JOB" (DATA_PLANE_ID, STATUS, CREATED_AT);
