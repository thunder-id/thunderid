// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Authentication flow graph identifiers that define the available authentication flows
 * supported by the platform.
 *
 * These graph IDs correspond to pre-configured authentication flow definitions in the backend
 * that orchestrate the authentication process based on the selected identity providers and
 * authentication mechanisms.
 *
 * @remarks
 * Each property represents a unique authentication flow configuration:
 * - `BASIC` - Standard username and password authentication flow
 * - `GOOGLE` - Google social login authentication flow
 * - `GITHUB` - GitHub social login authentication flow
 * - `BASIC_GOOGLE` - Combined username/password and Google authentication with user choice
 * - `BASIC_GOOGLE_GITHUB` - Combined username/password, Google, and GitHub authentication with user choice
 * - `BASIC_GOOGLE_GITHUB_SMS` - Multi-factor authentication combining basic, social, and SMS verification
 * - `BASIC_WITH_PROMPT` - Username/password authentication with additional user prompts or verification steps
 * - `SMS` - SMS-based passwordless authentication using one-time codes
 * - `SMS_WITH_USERNAME` - SMS authentication flow that requires username identification first
 *
 * @example
 * Configure basic username/password authentication:
 * ```tsx
 * import { AUTH_FLOW_GRAPHS } from './auth-flow-graphs';
 *
 * const authConfig = {
 *   flowGraphId: AUTH_FLOW_GRAPHS.BASIC,
 *   // ... other config
 * };
 * ```
 *
 * @example
 * Configure multi-provider authentication:
 * ```tsx
 * const multiAuthConfig = {
 *   flowGraphId: AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB,
 *   // ... other config
 * };
 * ```
 */
export const AUTH_FLOW_GRAPHS = {
  BASIC: 'auth_flow_config_basic',
  GOOGLE: 'auth_flow_config_google',
  GITHUB: 'auth_flow_config_github',
  BASIC_GOOGLE: 'auth_flow_config_basic_google',
  BASIC_GOOGLE_GITHUB: 'auth_flow_config_basic_google_github',
  BASIC_GOOGLE_GITHUB_SMS: 'auth_flow_config_basic_google_github_sms',
  BASIC_WITH_PROMPT: 'auth_flow_config_basic_with_prompt',
  SMS: 'auth_flow_config_sms',
  SMS_WITH_USERNAME: 'auth_flow_config_sms_with_username',
} as const;

/**
 * Registration flow graph identifiers that define the available user registration flows
 * supported by the IAM stack.
 *
 * These graph IDs correspond to pre-configured registration flow definitions in the backend
 * that orchestrate the user onboarding process based on the selected registration methods
 * and identity providers.
 *
 * @remarks
 * Each property represents a unique registration flow configuration:
 * - `BASIC` - Standard email/username and password registration flow
 * - `BASIC_GOOGLE_GITHUB` - Registration with email/password or social login (Google/GitHub) options
 * - `BASIC_GOOGLE_GITHUB_SMS` - Registration with email/password, social login, and SMS verification
 * - `SMS` - SMS-based registration using mobile number verification
 *
 * @example
 * Configure basic email/password registration:
 * ```tsx
 * import { REGISTRATION_FLOW_GRAPHS } from './auth-flow-graphs';
 *
 * const registrationConfig = {
 *   flowGraphId: REGISTRATION_FLOW_GRAPHS.BASIC,
 *   // ... other config
 * };
 * ```
 *
 * @example
 * Configure registration with social login options:
 * ```tsx
 * const socialRegistrationConfig = {
 *   flowGraphId: REGISTRATION_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB,
 *   // ... other config
 * };
 * ```
 */
export const REGISTRATION_FLOW_GRAPHS = {
  BASIC: 'registration_flow_config_basic',
  BASIC_GOOGLE_GITHUB: 'registration_flow_config_basic_google_github',
  BASIC_GOOGLE_GITHUB_SMS: 'registration_flow_config_basic_google_github_sms',
  SMS: 'registration_flow_config_sms',
} as const;
