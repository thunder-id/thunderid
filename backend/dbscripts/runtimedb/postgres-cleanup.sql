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
-- Stored procedure: purge all expired runtimedb rows.
--
-- Run once manually (ad-hoc / on-demand):
--   PGPASSWORD=<pass> psql -h <host> -p <port> -U <user> -d <runtimedb> \
--     -c "CALL cleanup_expired_runtimedb_data();"
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

CREATE OR REPLACE PROCEDURE cleanup_expired_runtimedb_data()
LANGUAGE plpgsql
AS $$
DECLARE
    v_now TIMESTAMP := NOW() AT TIME ZONE 'UTC';
BEGIN
    DELETE FROM "AUTHORIZATION_CODE"    WHERE EXPIRY_TIME < v_now;
    DELETE FROM "AUTHORIZATION_REQUEST" WHERE EXPIRY_TIME < v_now;
    DELETE FROM "CIBA_AUTH_REQUEST"     WHERE EXPIRY_TIME < v_now;
    DELETE FROM "WEBAUTHN_SESSION"      WHERE EXPIRY_TIME < v_now;
    DELETE FROM "PAR_REQUEST"           WHERE EXPIRY_TIME < v_now;
    DELETE FROM "JTI_RECORD"            WHERE EXPIRY_TIME < v_now;
    DELETE FROM "OPENID4VP_REQUEST_STATE"      WHERE EXPIRY_TIME < v_now;
    DELETE FROM "OPENID4VCI_NONCE"             WHERE EXPIRY_TIME < v_now;
    DELETE FROM "OPENID4VCI_CREDENTIAL_OFFER"  WHERE EXPIRY_TIME < v_now;
    DELETE FROM "RUNTIME_STORE"         WHERE EXPIRY_TIME < v_now;
END;
$$;
