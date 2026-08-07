#!/usr/bin/env bash
# ----------------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# ----------------------------------------------------------------------------

# Provision a Control Plane, once, before its pods are started.
#
# Run this from outside the deployment, from an operator's machine or a platform task, against an
# unpacked ThunderID Control Plane distribution holding this deployment's deployment.yaml, and with
# reach to its database. It is not part of the image and is not meant to run in a pod.
#
# Provisioning happens once. Re-running after a failure part way through is safe: a step already done
# is skipped.#
# Usage:
#   THUNDERID_HOME=/path/to/distribution \
#   ADMIN_PASSWORD=... ./setup-control-plane.sh
#
# or pass the distribution as --home /path/to/distribution.
#
#   Key material     TLS, signing, and encryption keys, plus the Direct API secret. Generated only
#                    when absent, so a redeploy keeps the keys already in use.
#   Database schema  Loaded into each of the four datasources that has none yet.
#   Root tenant      The baseline for the tenant that owns the Control Plane's own resources. In
#                    token mode nothing can call /system/tenants to create any other tenant until
#                    this exists, so it is provisioned here rather than left to be done by hand.
#
# deployment.yaml is read and never written. Mount the deployment's own configuration before running.
#
# Environment:
#   ADMIN_USERNAME  first administrator's username (default "admin")
#   ADMIN_PASSWORD  first administrator's password (required)
#   DB_CONFIG_PASSWORD, DB_RUNTIME_TRANSIENT_PASSWORD, DB_ENTITY_PASSWORD,
#   DB_RUNTIME_PERSISTENT_PASSWORD
#                   database passwords, for PostgreSQL. Take them from the same Secret the server
#                   itself reads: deployment.yaml holds placeholders, not the values.

set -euo pipefail

# shellcheck source=./plane-setup-common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/plane-setup-common.sh"

resolve_home "$@"

log "Provisioning the Control Plane"
require_config
expect_mode cp

# The schema comes first: seeding the baseline writes to these databases.
require_referenced_files
ensure_schema

# In token mode every request names its tenant in a token claim, so there is no server-wide tenant
# and the baseline has to be provisioned against a named one: the tenant owning the Control Plane's
# own resources, which is the one the platform APIs are served under.
#
# In server mode the deployment is single-tenant, and the baseline belongs to the server itself.
DEPLOYMENT_SOURCE="$(yaml_get server.deployment_id_source)"
if [ "$DEPLOYMENT_SOURCE" = "token" ]; then
    ROOT_TENANT="$(yaml_get server.system_deployment_id)"
    [ -n "$ROOT_TENANT" ] || die "server.system_deployment_id is not set in $CONFIG_FILE.
In token mode it names the tenant owning the Control Plane's own resources, which is the one
provisioned here and the one the platform APIs are served under."
    run_bootstrap "$ROOT_TENANT"
else
    run_bootstrap
fi

log "Control Plane provisioned."
echo
echo "Further tenants are created through the Control Plane's own /system/tenants API, which"
echo "provisions each one's baseline. Do not rerun this script for them."
