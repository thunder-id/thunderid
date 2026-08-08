#!/usr/bin/env bash
# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0
# Benchmark script to compare ESLint vs oxlint execution speed.

set -euo pipefail

echo "=========================================="
echo "⚡ Benchmarking oxlint (non-type-aware) vs ESLint (type-aware)"
echo "=========================================="

RUNS=3
FAILED_RUNS=0

echo ""
echo "--- Running oxlint (non-type-aware, $RUNS iterations) ---"
for i in $(seq 1 $RUNS); do
  echo "Run $i:"
  set +e
  time pnpm exec oxlint -c frontend/.oxlintrc.json frontend > /dev/null 2>&1
  STATUS=$?
  set -e
  if [ $STATUS -ne 0 ]; then
    echo "Run $i exited with code $STATUS"
    FAILED_RUNS=$((FAILED_RUNS + 1))
  fi
done

echo ""
echo "--- Running ESLint (via Turbo) ($RUNS iterations) ---"
for i in $(seq 1 $RUNS); do
  echo "Run $i:"
  set +e
  time NODE_OPTIONS="--max-old-space-size=8192" pnpm turbo run lint --filter="./frontend/**" > /dev/null 2>&1
  STATUS=$?
  set -e
  if [ $STATUS -ne 0 ]; then
    echo "Run $i exited with code $STATUS"
    FAILED_RUNS=$((FAILED_RUNS + 1))
  fi
done

echo ""
if [ $FAILED_RUNS -gt 0 ]; then
  echo "Benchmark completed with $FAILED_RUNS non-zero exit run(s)."
  exit 1
fi
echo "Benchmark completed."
