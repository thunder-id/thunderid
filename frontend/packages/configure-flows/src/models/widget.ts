// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Base} from './base';

export type Widget = Base<WidgetExtendedConfig>;

/**
 * Interface for the properties of a widget.
 */
export interface WidgetExtendedConfig {
  /**
   * Version of the widget.
   */
  version?: string;
  data: unknown;
}

export const WidgetCategories = {
  Composite: 'COMPOSITE',
  Flow: 'FLOW',
  Security: 'SECURITY',
} as const;

export type WidgetCategories = (typeof WidgetCategories)[keyof typeof WidgetCategories];

export const WidgetTypes = {
  IdentifierPassword: 'IDENTIFIER_PASSWORD',
  SMSOTP: 'SMS_OTP',
  EmailOTP: 'EMAIL_OTP',
  GoogleFederation: 'GOOGLE_FEDERATION',
  GithubFederation: 'GITHUB_FEDERATION',
  EUDIWallet: 'EUDI_WALLET',
  PasskeyAuthentication: 'PASSKEY_AUTHENTICATION',
  Provisioning: 'PROVISIONING',
  Consent: 'CONSENT',
  MagicLink: 'MAGIC_LINK',
  SelfSignUpLink: 'SELF_SIGN_UP_LINK',
  SignInLink: 'SIGN_IN_LINK',
  RecoveryLink: 'RECOVERY_LINK',
} as const;

export type WidgetTypes = (typeof WidgetTypes)[keyof typeof WidgetTypes];
