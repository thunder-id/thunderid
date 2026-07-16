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
-- Stored procedure: purge expired runtimedb rows in bounded batches.
--
-- Deletes expired rows in batches of p_batch_size (default 1000), committing
-- after each batch to keep locks short on large tables. Must run as a top-level
-- CALL (the per-batch COMMIT cannot run inside an outer transaction).
--
-- Run once manually (ad-hoc / on-demand):
--   PGPASSWORD=<pass> psql -h <host> -p <port> -U <user> -d <runtimedb> \
--     -c "CALL cleanup_expired_runtimedb_data();"
--
--   -- Optional: override the batch size (rows deleted per batch):
--   -c "CALL cleanup_expired_runtimedb_data(500);"
--
-- Scheduled execution options:
--
--   1. pg_cron (RECOMMENDED, requires the pg_cron extension):
--      CREATE EXTENSION IF NOT EXISTS pg_cron;
--      SELECT cron.schedule(
--        'cleanup-runtimedb-expired',
--        '*/60 * * * *',
--        $$CALL cleanup_expired_runtimedb_data()$$
--      );
--      -- To verify: SELECT * FROM cron.job WHERE jobname = 'cleanup-runtimedb-expired';
--      -- To remove: SELECT cron.unschedule('cleanup-runtimedb-expired');
--
--   2. Kubernetes CronJob: call CALL cleanup_expired_runtimedb_data()
--      via a psql container on the desired schedule.
--
--   3. OS cron (every 60 minutes):
-- --      */60 * * * * postgres PGPASSWORD=<pass> psql -h <host> -p <port> \
-- --        -U <user> -d <runtimedb> -c "CALL cleanup_expired_runtimedb_data();" \
-- --        >> /var/log/thunderid-cleanup.log 2>&1
-- ============================================================

-- Drop the old parameterless signature so re-applying doesn't leave an ambiguous overload.
DROP PROCEDURE IF EXISTS cleanup_expired_runtimedb_data();

CREATE OR REPLACE PROCEDURE cleanup_expired_runtimedb_data(p_batch_size INT DEFAULT 1000)
LANGUAGE plpgsql
AS $$
DECLARE
    v_now     TIMESTAMP := NOW() AT TIME ZONE 'UTC';
    v_deleted INT;
    v_table   TEXT;
    -- Tables located by ctid. Safe because each is a single, non-partitioned
    -- relation, so ctid uniquely identifies a row within it.
    v_ctid_tables TEXT[] := ARRAY[
        'AUTHORIZATION_CODE',
        'AUTHORIZATION_REQUEST',
        'CIBA_AUTH_REQUEST',
        'WEBAUTHN_SESSION',
        'PAR_REQUEST',
        'JTI_RECORD',
        'OPENID4VP_REQUEST_STATE',
        'OPENID4VCI_NONCE',
        'OPENID4VCI_CREDENTIAL_OFFER'
    ];
BEGIN
    -- Guard against a batch size that would disable batching or make no progress.
    IF p_batch_size IS NULL OR p_batch_size <= 0 THEN
        p_batch_size := 1000;
    END IF;

    FOREACH v_table IN ARRAY v_ctid_tables LOOP
        LOOP
            EXECUTE format(
                'DELETE FROM %I WHERE ctid IN ' ||
                '(SELECT ctid FROM %I WHERE EXPIRY_TIME < $1 LIMIT $2)',
                v_table, v_table
            ) USING v_now, p_batch_size;
            GET DIAGNOSTICS v_deleted = ROW_COUNT;
            COMMIT;
            EXIT WHEN v_deleted = 0;
        END LOOP;
    END LOOP;

    -- RUNTIME_STORE is LIST-partitioned by NAMESPACE, where ctid is not unique across
    -- partitions; match rows by primary key rather than ctid. ORDER BY EXPIRY_TIME lets
    -- the batch be located via an index scan.
    LOOP
        DELETE FROM "RUNTIME_STORE"
        WHERE (DEPLOYMENT_ID, NAMESPACE, KEY) IN (
            SELECT DEPLOYMENT_ID, NAMESPACE, KEY FROM "RUNTIME_STORE"
            WHERE EXPIRY_TIME < v_now ORDER BY EXPIRY_TIME LIMIT p_batch_size
        );
        GET DIAGNOSTICS v_deleted = ROW_COUNT;
        COMMIT;
        EXIT WHEN v_deleted = 0;
    END LOOP;
END;
$$;
