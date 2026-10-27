#!/bin/bash
# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Entrypoint of the quickstart image. Runs the pack's one-time setup on first start, then starts
# the three planes. Setup is skipped once it has completed, so key material and databases kept on
# a volume mounted at /opt/thunderid-quickstart survive restarts.
#
# Setup is not interactive here. Set ADMIN_USERNAME, ADMIN_PASSWORD, SERVICE_DESK_USERNAME and
# SERVICE_DESK_PASSWORD to choose the accounts; any password left unset is generated and printed
# in the container log.

set -e

cd /opt/thunderid-quickstart

if [ ! -f .quickstart-setup-done ]; then
    ./setup.sh
fi

exec ./start.sh
