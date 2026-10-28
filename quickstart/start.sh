#!/bin/bash
# Copyright 2025-2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Quickstart start: starts the three deployments that setup.sh prepared: auth-dp (the Control
# Plane's own sign-in), the managed Data Plane, and the Control Plane. Run setup.sh once first.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -f ".quickstart-setup-done" ]; then
    echo "❌ The quickstart has not been set up yet. Run ./setup.sh first."
    exit 1
fi

# shellcheck source=quickstart/bootstrap.sh
source "$SCRIPT_DIR/quickstart-bootstrap.sh"

cleaned=false
cleanup() {
    $cleaned && return 0
    cleaned=true
    echo -e "\n🛑 Stopping Control Plane, Data Plane and auth-dp..."
    kill_port "$CP_PORT"
    kill_port "$DP_PORT"
    kill_port "$AUTH_DP_PORT"
    wait 2>/dev/null || true
    echo "✅ Stopped."
}
trap cleanup INT TERM EXIT

kill_port "$CP_PORT"
kill_port "$DP_PORT"
kill_port "$AUTH_DP_PORT"

echo "========================================="
echo "   ThunderID Quickstart"
echo "========================================="
echo ""
echo "⚡ Starting auth-dp..."
(cd auth-dp && ./start.sh) &

echo "⚡ Starting the Data Plane..."
(cd dp && ./start.sh) &

echo "⚡ Starting the Control Plane..."
(cd cp && ./start-cp.sh) &

sleep 2

echo ""
echo "🚀 Quickstart running:"
echo "  🔐 auth-dp (Control Plane sign-in):"
echo "      🚪 Gate (Login/Register): https://localhost:$AUTH_DP_PORT/gate"
echo "  📱 Data Plane (managed):"
echo "      🚪 Gate (Login/Register): https://localhost:$DP_PORT/gate"
echo "      🛠️  Admin Console (Users, Agents, Groups, Roles): https://localhost:$DP_PORT/admin"
echo "  🛠️  Control Plane Console: https://localhost:$CP_PORT/console"
echo ""
echo "  The Data Plane is already registered as a gateway on the Control Plane: an application,"
echo "  connection, role, or flow created on the Control Plane is captured and applied to the"
echo "  Data Plane automatically (gateway.auto_apply, on for this pack alone). Signing into the"
echo "  Control Plane's console authenticates against auth-dp, a separate deployment, not the"
echo "  Data Plane it manages. Neither plane's own console shows applications/connections/"
echo "  roles/flows (those are authored on the Control Plane). Check the Data Plane's own API,"
echo "  or GET /configuration-versions and GET /gateways/{id}/applied-version on the Control"
echo "  Plane, to see what was synced."
echo ""
echo "Press Ctrl+C to stop all three."

wait
