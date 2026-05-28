#!/bin/bash
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

# Starts the three Wayfinder sample services: the Node backend (REST API on
# /api/* and MCP server on /mcp), AI chat agent, and React frontend. Streams
# each service's logs to a file under logs/ and prints aggregated status
# to stdout.

set -e

BACKEND_PORT=8787
AGENT_PORT=8790
FRONTEND_PORT=5173

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

mkdir -p logs

function kill_port() {
    local port=$1
    lsof -ti tcp:"$port" | xargs kill -9 2>/dev/null || true
}

for p in $BACKEND_PORT $AGENT_PORT $FRONTEND_PORT; do
    kill_port "$p"
done

if ! command -v npm >/dev/null 2>&1; then
    echo "❌ Error: npm is not installed. Please install Node.js 20+ and npm."
    exit 1
fi

function ensure_install() {
    local dir=$1
    if [ ! -d "$dir/node_modules" ]; then
        echo "📦 Installing dependencies in $dir..."
        (cd "$dir" && npm install --silent)
    fi
}

ensure_install backend
ensure_install ai-agent
ensure_install frontend

if [ ! -f "backend/wayfinder.sqlite" ]; then
    echo "🌱 Seeding backend database..."
    (cd backend && npm run seed)
fi

echo "⚡ Starting Wayfinder services..."

(cd backend  && npm start > "$SCRIPT_DIR/logs/backend.log" 2>&1) &
BACKEND_PID=$!
(cd ai-agent && npm start > "$SCRIPT_DIR/logs/ai-agent.log" 2>&1) &
AGENT_PID=$!
(cd frontend && npm run dev > "$SCRIPT_DIR/logs/frontend.log" 2>&1) &
FRONTEND_PID=$!

function shutdown() {
    echo ""
    echo "🛑 Stopping Wayfinder services..."
    kill $BACKEND_PID $AGENT_PID $FRONTEND_PID 2>/dev/null || true
    for p in $BACKEND_PORT $AGENT_PORT $FRONTEND_PORT; do
        kill_port "$p"
    done
    exit 0
}

trap shutdown SIGINT SIGTERM

echo ""
echo "🚀 Wayfinder sample is starting up. Logs under ./logs/"
echo "   - Travel REST API: http://localhost:$BACKEND_PORT       (logs: logs/backend.log)"
echo "   - MCP server:      http://localhost:$BACKEND_PORT/mcp   (logs: logs/backend.log)"
echo "   - AI chat agent:   http://localhost:$AGENT_PORT/chat  (logs: logs/ai-agent.log)"
echo "   - Frontend:        http://localhost:$FRONTEND_PORT       (logs: logs/frontend.log)"
echo ""
echo "Press Ctrl+C to stop all services."

wait $BACKEND_PID $AGENT_PID $FRONTEND_PID
