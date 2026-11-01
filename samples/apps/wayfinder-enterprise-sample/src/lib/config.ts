// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import 'server-only';

const required = (name: string): string => {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} is not set. Copy .env.example to .env.local and fill it in.`);
  }
  return value;
};

/** Server-side settings, read from the environment. */
const config = {
  get thunderIdUrl(): string {
    return required('NEXT_PUBLIC_THUNDERID_BASE_URL').replace(/\/+$/, '');
  },
  get clientId(): string {
    return required('NEXT_PUBLIC_THUNDERID_CLIENT_ID');
  },
  get appUrl(): string {
    return required('WAYFINDER_APP_URL').replace(/\/+$/, '');
  },
  get apiResource(): string {
    return required('WAYFINDER_API_RESOURCE');
  },
  get signInFlowId(): string {
    return required('WAYFINDER_SIGN_IN_FLOW_ID');
  },
  get adminRoleId(): string {
    return required('WAYFINDER_ADMIN_ROLE_ID');
  },
  get memberRoleId(): string {
    return required('WAYFINDER_MEMBER_ROLE_ID');
  },
  get adminRoleName(): string {
    return required('WAYFINDER_ADMIN_ROLE_NAME');
  },
  get userType(): string {
    return required('WAYFINDER_USER_TYPE');
  },
  get backendClientId(): string {
    return required('WAYFINDER_BACKEND_CLIENT_ID');
  },
  get backendClientSecret(): string {
    return required('WAYFINDER_BACKEND_CLIENT_SECRET');
  },
  get systemResource(): string {
    return required('THUNDERID_SYSTEM_RESOURCE');
  },
};

export default config;
