// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/// <reference types="vite/client" />
/// <reference types="vite-plugin-svgr/client" />

declare global {
  const __DEV_SERVER_URL__: string;
  const __DEV_GATE_URL__: string;
  const __DEV_CLIENT_ID__: string;
}

export {};
