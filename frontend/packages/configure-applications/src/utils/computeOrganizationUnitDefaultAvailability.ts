// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {OrganizationUnitDefaultItem} from '../models/application-create-flow';

export interface OrganizationUnitDefaultAvailabilityFlowFlags {
  /** Whether the selected template's flow includes a Security step. */
  hasSecurityStep: boolean;
  /** Whether the selected template's flow includes a Design step. */
  hasDesignStep: boolean;
}

/**
 * Which organization-unit-defaultable items are actually offerable: the OU has a configured value
 * for the item, *and* the selected template's flow has a step for it in the first place (a
 * template with no Security step never presents a hosted sign-in screen to inherit one for, and
 * one with no Design step has no theme/layout of its own). Shared between
 * `OrganizationUnitDefaultsSection` (which item rows to show) and `ConfigureApplicationDetails`
 * (which items to pre-check the first time an OU resolves).
 */
const computeOrganizationUnitDefaultAvailability = (
  organizationUnit: OrganizationUnit | undefined,
  {hasSecurityStep, hasDesignStep}: OrganizationUnitDefaultAvailabilityFlowFlags,
): Record<OrganizationUnitDefaultItem, boolean> => ({
  [OrganizationUnitDefaultItem.SIGN_IN]: hasSecurityStep && Boolean(organizationUnit?.authFlowId),
  [OrganizationUnitDefaultItem.SIGN_UP]: hasSecurityStep && Boolean(organizationUnit?.registrationFlowId),
  [OrganizationUnitDefaultItem.RECOVERY]: hasSecurityStep && Boolean(organizationUnit?.recoveryFlowId),
  [OrganizationUnitDefaultItem.SIGN_OUT]: hasSecurityStep && Boolean(organizationUnit?.signOutFlowId),
  [OrganizationUnitDefaultItem.THEME]: hasDesignStep && Boolean(organizationUnit?.themeId),
  [OrganizationUnitDefaultItem.LAYOUT]: hasDesignStep && Boolean(organizationUnit?.layoutId),
});

export default computeOrganizationUnitDefaultAvailability;
