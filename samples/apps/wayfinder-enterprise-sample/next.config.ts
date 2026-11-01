// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  // Stop `next dev` from writing its AGENTS.md block for AI coding agents.
  agentRules: false,
};

export default nextConfig;
