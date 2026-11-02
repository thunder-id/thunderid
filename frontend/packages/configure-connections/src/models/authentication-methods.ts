// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

export const AuthenticationMethods = {
  NONE: 'NONE',
  BEARER: 'BEARER',
  BASIC: 'BASIC',
  API_KEY: 'API_KEY',
} as const;

export type AuthenticationMethod = (typeof AuthenticationMethods)[keyof typeof AuthenticationMethods];

export const AUTHENTICATION_METHOD_FIELD_NAMES = new Set([
  'authenticationScheme',
  'bearerToken',
  'basicUsername',
  'basicPassword',
  'httpHeaders',
]);
