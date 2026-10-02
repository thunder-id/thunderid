// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The query parameter that opens a tab of a gateway's page, and the tabs it can open.
 */
export const GATEWAY_DETAIL_TAB_PARAM = 'tab';
export const GatewayDetailTabs = {
  VARIABLES: 'variables',
  SECRETS: 'secrets',
} as const;
export type GatewayValuesTab = (typeof GatewayDetailTabs)[keyof typeof GatewayDetailTabs];

/**
 * The rules a gateway's store applies to a variable or a secret.
 */
export const GATEWAY_VALUE_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;
export const GATEWAY_VALUE_NAME_MAX_LENGTH = 255;
export const GATEWAY_VALUE_MAX_LENGTH = 8192;
export const GATEWAY_VALUE_DESCRIPTION_MAX_LENGTH = 1000;
