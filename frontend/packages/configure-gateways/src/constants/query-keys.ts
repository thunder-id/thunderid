// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const DeployQueryKeys = {
  GATEWAYS: 'deploy-gateways',
  VERSIONS: 'deploy-versions',
  VERSION: 'deploy-version',
  APPLIED_VERSION: 'deploy-applied-version',
  APPLIED_CONFIGURATION: 'deploy-applied-configuration',
  VALUES: 'deploy-values',
  APPLY_CHECK: 'deploy-apply-check',
  RUNNING_REFERENCES: 'deploy-running-references',
} as const;

export default DeployQueryKeys;
