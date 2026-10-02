// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const GatewayQueryKeys = {
  GATEWAYS: 'gateways',
  GATEWAY: 'gateway',
  APPLIED_VERSION: 'gateway-applied-version',
  GATEWAY_DIFF: 'gateway-diff',
  CONFIGURATION_VERSIONS: 'configuration-versions',
  GATEWAY_DRY_RUN: 'gateway-dry-run',
  GATEWAY_VARIABLES: 'gateway-variables',
  GATEWAY_SECRETS: 'gateway-secrets',
} as const;

export default GatewayQueryKeys;
