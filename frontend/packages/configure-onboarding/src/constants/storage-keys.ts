// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Storage key suffix constants for the welcome feature.
 * The product name prefix is prepended at runtime via the utility functions.
 *
 * Utility functions in the `utils` directory generate the full storage keys by replacing the `{{productName}}`
 * placeholder with the actual product name at runtime.
 */
const WelcomeStorageKeys = {
  DISMISSED: '{{productName}}:welcome:dismissed',
  SESSION_CHECKED: '{{productName}}:welcome:session-checked',
  WAYFINDER_CONFIGURED: '{{productName}}:wayfinder-config-imported',
  WAYFINDER_SETUP_EXPANDED: '{{productName}}:wayfinder-setup-expanded',
} as const;

export default WelcomeStorageKeys;
