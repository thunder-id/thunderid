// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Application creation step identifiers used in application creation flow
 * to track the current step and navigate between steps.
 *
 * @public
 */
export const ApplicationCreateFlowStep = {
  ORGANIZATION_UNIT: 'ORGANIZATION_UNIT',
  DETAILS: 'DETAILS',
  SECURITY: 'SECURITY',
  DESIGN: 'DESIGN',
  CONFIGURE: 'CONFIGURE',
  CLIENT_TYPE: 'CLIENT_TYPE',
  COMPLETE: 'COMPLETE',
} as const;

/**
 * Sign-in approach identifiers for application creation flow.
 *
 * @public
 */
export const ApplicationCreateFlowSignInApproach = {
  REDIRECT_BASED: 'REDIRECT_BASED',
  EMBEDDED: 'EMBEDDED',
} as const;

/**
 * Configuration type identifiers for application creation flow.
 *
 * @public
 */
export const ApplicationCreateFlowConfiguration = {
  URL: 'URL',
  DEEPLINK: 'DEEPLINK',
  NONE: 'NONE',
} as const;

/**
 * Application creation step type
 *
 * @public
 */
export type ApplicationCreateFlowStep = keyof typeof ApplicationCreateFlowStep;

/**
 * Sign-in approach type
 *
 * @public
 */
export type ApplicationCreateFlowSignInApproach = keyof typeof ApplicationCreateFlowSignInApproach;

/**
 * Configuration type
 *
 * @public
 */
export type ApplicationCreateFlowConfiguration = keyof typeof ApplicationCreateFlowConfiguration;

/**
 * The organization-unit-defaultable items shown in the Details step's "Use organization unit
 * defaults" accordion.
 *
 * @public
 */
export const OrganizationUnitDefaultItem = {
  SIGN_IN: 'signIn',
  SIGN_UP: 'signUp',
  RECOVERY: 'recovery',
  SIGN_OUT: 'signOut',
  THEME: 'theme',
  LAYOUT: 'layout',
} as const;

/**
 * Organization-unit-defaultable item type
 *
 * @public
 */
export type OrganizationUnitDefaultItem =
  (typeof OrganizationUnitDefaultItem)[keyof typeof OrganizationUnitDefaultItem];

/**
 * Per-item "use the organization unit's default" selection backing the Details step's accordion.
 *
 * @public
 */
export type OrganizationUnitDefaultsSelection = Record<OrganizationUnitDefaultItem, boolean>;
