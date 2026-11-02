// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Constants for preview screen types.
 */
export const PreviewScreenType = {
  COMMON: 'common',
  LOGIN: 'login',
  MY_ACCOUNT: 'myaccount',
  EMAIL_LINK_EXPIRY: 'email-link-expiry',
  EMAIL_TEMPLATE: 'email-template',
  SIGN_UP: 'sign-up',
  EMAIL_OTP: 'email-otp',
  PUSH_AUTH: 'push-auth',
  SMS_OTP: 'sms-otp',
  TOTP: 'totp',
  PASSWORD_RECOVERY: 'password-recovery',
  PASSWORD_RESET: 'password-reset',
  PASSWORD_RESET_SUCCESS: 'password-reset-success',
  USERNAME_RECOVERY_CLAIM: 'username-recovery-claim',
  USERNAME_RECOVERY_CHANNEL_SELECTION: 'username-recovery-channel-selection',
  USERNAME_RECOVERY_SUCCESS: 'username-recovery-success',
} as const;

export type PreviewScreenType = (typeof PreviewScreenType)[keyof typeof PreviewScreenType];

/**
 * Interface for the base custom text hook result.
 */
export interface CustomTextPreferenceResult {
  /**
   * The data returned by the hook.
   */
  data?: Partial<Record<PreviewScreenType, Record<string, string>>>;
  /**
   * Error state.
   */
  error?: unknown;
  /**
   * Loading state.
   */
  isLoading: boolean;
  /**
   * Mutate function to revalidate data.
   */
  mutate: () => void;
}

/**
 * Interface for the screen meta hook result.
 */
export interface CustomTextPreferenceScreenMetaResult {
  /**
   * The data returned by the hook.
   */
  data?: Partial<Record<PreviewScreenType, CustomTextPreferenceScreenMetaInterface>>;
  /**
   * Error state.
   */
  error?: unknown;
  /**
   * Loading state.
   */
  isLoading: boolean;
  /**
   * Mutate function to revalidate data.
   */
  mutate: () => void;
}

/**
 * Interface for the custom text preference screen meta.
 */
export type CustomTextPreferenceScreenMetaInterface = Record<
  string,
  {
    /**
     * Is the text preference editable.
     */
    EDITABLE: boolean;
    /**
     * Screen name of the text preference.
     */
    SCREEN: string;
    /**
     * Is the text preference multi-line.
     */
    MULTI_LINE: boolean;
  }
>;
