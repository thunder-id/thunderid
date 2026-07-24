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

-- ============================================================
-- Stored procedure: purge expired runtime_persistent rows in bounded batches.
--
-- Unlike runtime_transient, runtime_persistent data is authoritative and must survive a
-- runtime_transient flush; only rows past their EXPIRY_TIME are safe to delete. A revoked
-- token's row is removable once the token itself would have naturally expired.
--
-- Deletes expired rows in batches of p_batch_size (default 1000), committing
-- after each batch to keep locks short on large tables. Must run as a top-level
-- CALL (the per-batch COMMIT cannot run inside an outer transaction).
--
-- Run once manually (ad-hoc / on-demand):
--   PGPASSWORD=<pass> psql -h <host> -p <port> -U <user> -d <runtime_persistent> \
--     -c "CALL cleanup_expired_runtime_persistent_data();"
--
--   -- Optional: override the batch size (rows deleted per batch):
--   -c "CALL cleanup_expired_runtime_persistent_data(500);"
--
-- Scheduled execution options:
--
--   1. pg_cron (RECOMMENDED, requires the pg_cron extension):
--      CREATE EXTENSION IF NOT EXISTS pg_cron;
--      SELECT cron.schedule(
--        'cleanup-runtime_persistent-expired',
--        '*/60 * * * *',
--        $$CALL cleanup_expired_runtime_persistent_data()$$
--      );
--      -- To verify: SELECT * FROM cron.job WHERE jobname = 'cleanup-runtime_persistent-expired';
--      -- To remove: SELECT cron.unschedule('cleanup-runtime_persistent-expired');
--
--   2. Kubernetes CronJob: call CALL cleanup_expired_runtime_persistent_data()
--      via a psql container on the desired schedule.
--
--   3. OS cron (every 60 minutes):
-- --      */60 * * * * postgres PGPASSWORD=<pass> psql -h <host> -p <port> \
-- --        -U <user> -d <runtime_persistent> -c "CALL cleanup_expired_runtime_persistent_data();" \
-- --        >> /var/log/thunderid-operation-cleanup.log 2>&1
-- ============================================================

-- Drop the old parameterless signature so re-applying doesn't leave an ambiguous overload.
DROP PROCEDURE IF EXISTS cleanup_expired_runtime_persistent_data();

CREATE OR REPLACE PROCEDURE cleanup_expired_runtime_persistent_data(p_batch_size INT DEFAULT 1000)
LANGUAGE plpgsql
AS $$
DECLARE
    v_now     TIMESTAMP := NOW() AT TIME ZONE 'UTC';
    v_deleted INT;
BEGIN
    -- Guard against a batch size that would disable batching or make no progress.
    IF p_batch_size IS NULL OR p_batch_size <= 0 THEN
        p_batch_size := 1000;
    END IF;

    LOOP
        DELETE FROM "REVOKED_TOKEN"
        WHERE ctid IN (
            SELECT ctid FROM "REVOKED_TOKEN" WHERE EXPIRY_TIME < v_now LIMIT p_batch_size
        );
        GET DIAGNOSTICS v_deleted = ROW_COUNT;
        COMMIT;
        EXIT WHEN v_deleted = 0;
    END LOOP;

    -- SSO sessions past their absolute deadline, together with their context and participant
    -- children. A session is live only while now < IDLE_EXPIRES_AT AND now < ABSOLUTE_EXPIRES_AT, so
    -- a row past ABSOLUTE_EXPIRES_AT can never resume and is safe to delete; idle-expired-but-absolute-
    -- live rows are left for a later sweep (the resolver already rejects them). Sweeping by
    -- ABSOLUTE_EXPIRES_AT (immutable and indexed) keeps this an index-backed scan over cold rows and
    -- leaves the mutable IDLE_EXPIRES_AT unindexed so the hot activity touch stays HOT-eligible.
    -- There is no FK cascade between the three SSO tables, so each batch deletes the children
    -- explicitly using the same victim set as the parents; the victim set is located via
    -- idx_sso_session_absolute_expires_at (ORDER BY drives the index scan) and all data-modifying
    -- CTEs run against the statement-start snapshot, so delete order among them is irrelevant.
    LOOP
        WITH victims AS (
            SELECT SESSION_ID, DEPLOYMENT_ID
            FROM "SSO_SESSION"
            WHERE ABSOLUTE_EXPIRES_AT <= v_now
            ORDER BY ABSOLUTE_EXPIRES_AT
            LIMIT p_batch_size
        ),
        del_ctx AS (
            DELETE FROM "SSO_SESSION_CONTEXT" c
            USING victims v
            WHERE c.SESSION_ID = v.SESSION_ID AND c.DEPLOYMENT_ID = v.DEPLOYMENT_ID
        ),
        del_part AS (
            DELETE FROM "SSO_SESSION_PARTICIPANT" p
            USING victims v
            WHERE p.SESSION_ID = v.SESSION_ID AND p.DEPLOYMENT_ID = v.DEPLOYMENT_ID
        ),
        del_sess AS (
            DELETE FROM "SSO_SESSION" s
            USING victims v
            WHERE s.SESSION_ID = v.SESSION_ID AND s.DEPLOYMENT_ID = v.DEPLOYMENT_ID
            RETURNING 1
        )
        SELECT COUNT(*) INTO v_deleted FROM del_sess;
        COMMIT;
        EXIT WHEN v_deleted = 0;
    END LOOP;
END;
$$;
