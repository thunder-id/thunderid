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
-- Stored procedure: purge expired operationdb rows in bounded batches.
--
-- Unlike runtimedb, operation data is authoritative and must survive a
-- runtime flush; only rows past their EXPIRY_TIME are safe to delete. A revoked
-- token's row is removable once the token itself would have naturally expired.
--
-- Deletes expired rows in batches of p_batch_size (default 1000), committing
-- after each batch to keep locks short on large tables. Must run as a top-level
-- CALL (the per-batch COMMIT cannot run inside an outer transaction).
--
-- Run once manually (ad-hoc / on-demand):
--   PGPASSWORD=<pass> psql -h <host> -p <port> -U <user> -d <operationdb> \
--     -c "CALL cleanup_expired_operationdb_data();"
--
--   -- Optional: override the batch size (rows deleted per batch):
--   -c "CALL cleanup_expired_operationdb_data(500);"
--
-- Scheduled execution options:
--
--   1. pg_cron (RECOMMENDED, requires the pg_cron extension):
--      CREATE EXTENSION IF NOT EXISTS pg_cron;
--      SELECT cron.schedule(
--        'cleanup-operationdb-expired',
--        '*/60 * * * *',
--        $$CALL cleanup_expired_operationdb_data()$$
--      );
--      -- To verify: SELECT * FROM cron.job WHERE jobname = 'cleanup-operationdb-expired';
--      -- To remove: SELECT cron.unschedule('cleanup-operationdb-expired');
--
--   2. Kubernetes CronJob: call CALL cleanup_expired_operationdb_data()
--      via a psql container on the desired schedule.
--
--   3. OS cron (every 60 minutes):
-- --      */60 * * * * postgres PGPASSWORD=<pass> psql -h <host> -p <port> \
-- --        -U <user> -d <operationdb> -c "CALL cleanup_expired_operationdb_data();" \
-- --        >> /var/log/thunderid-operation-cleanup.log 2>&1
-- ============================================================

-- Drop the old parameterless signature so re-applying doesn't leave an ambiguous overload.
DROP PROCEDURE IF EXISTS cleanup_expired_operationdb_data();

CREATE OR REPLACE PROCEDURE cleanup_expired_operationdb_data(p_batch_size INT DEFAULT 1000)
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
END;
$$;
