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

# Provision a Data Plane environment, once, when it is first stood up.
#
# Run this from outside the deployment, from an operator's machine or a platform task, against an
# unpacked ThunderID Data Plane distribution holding this environment's deployment.yaml, and with
# reach to its database. It is not part of the image and is not meant to run in a pod.
#
# One environment, for example org1's dev, is one Data Plane: run this once per environment, not once
# per replica. Re-running after a failure part way through is safe.
#
# All it does is load the database schema. A Data Plane is fed by a Control Plane: its organization
# units, user types, applications, flows and themes all arrive on the first apply, so there is no
# baseline to seed here and nothing that would leave a second copy for that apply to sit alongside.
# Key material is generated and mounted by whatever provisions the deployment, and every secret comes
# from the environment.
#
# deployment.yaml is read and never written.
#
# This does not register the environment with the Control Plane. That happens on the Control Plane,
# which issues this deployment's channel token and shows it once; put that token where
# channel.client.auth_token reads it from before starting the pods.
#
# Usage:
#   THUNDERID_HOME=/path/to/distribution ./setup-data-plane.sh
#
# or pass the distribution as --home /path/to/distribution.
#
# Environment:
#   DB_CONFIG_PASSWORD, DB_RUNTIME_TRANSIENT_PASSWORD, DB_ENTITY_PASSWORD,
#   DB_RUNTIME_PERSISTENT_PASSWORD
#                   database passwords, for PostgreSQL. Take them from the same Secret the server
#                   itself reads: deployment.yaml holds placeholders, not the values.

set -euo pipefail

# shellcheck source=./plane-setup-common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/plane-setup-common.sh"

resolve_home "$@"

log "Provisioning a Data Plane"
require_config
expect_mode dp

ensure_schema

# No baseline is seeded. A Data Plane is fed by a Control Plane, so every resource it serves arrives
# on the first apply; seeding anything here would leave a second copy for that apply to sit alongside.
# The schema above is all this deployment needs before the Control Plane reaches it.

# Warn rather than fail: a deployment may be provisioned before the environment is registered on the
# Control Plane, and the token put in place afterwards.
if [ "$(yaml_get channel.client.enabled)" != "true" ]; then
    warn "channel.client.enabled is not true, so this deployment will not connect to a Control Plane
and nothing can be applied to it. Set it, along with channel.client.id and control_plane_url."
fi

log "Data Plane provisioned."
echo
echo "Apply configuration to it from the Control Plane once its pods are connected."
