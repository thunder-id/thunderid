// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// APIs
export {default as useGetSharingPolicies} from './api/useGetSharingPolicies';
export {default as useSaveSharingPolicy} from './api/useSaveSharingPolicy';

// Components
export {default as AccessChoiceCards} from './components/AccessChoiceCards';
export type {AccessChoice, AccessChoiceCardsProps} from './components/AccessChoiceCards';
export {default as GrantedOrganizationUnitList} from './components/GrantedOrganizationUnitList';
export type {GrantedOrganizationUnit, GrantedOrganizationUnitListProps} from './components/GrantedOrganizationUnitList';
export {default as OrganizationAccessSettings} from './components/OrganizationAccessSettings';
export type {OrganizationAccessSettingsProps} from './components/OrganizationAccessSettings';
export {default as OrganizationUnitChipList} from './components/OrganizationUnitChipList';
export type {OrganizationUnitChipListProps} from './components/OrganizationUnitChipList';
export {default as OrganizationUnitSelectDialog} from './components/OrganizationUnitSelectDialog';
export type {
  OrganizationUnitSelectDialogProps,
  OrganizationUnitSelection,
  OrganizationUnitSource,
} from './components/OrganizationUnitSelectDialog';
export {default as SwitchAccessConfirmDialog} from './components/SwitchAccessConfirmDialog';
export type {SwitchAccessConfirmDialogProps} from './components/SwitchAccessConfirmDialog';

// Constants
export {default as ResourceSharingQueryKeys} from './constants/resource-sharing-query-keys';

// Hooks
export {default as useOrganizationAccess} from './hooks/useOrganizationAccess';
export type {OrganizationAccessState, UseOrganizationAccessOptions} from './hooks/useOrganizationAccess';

// Models
export type {
  AccessSwitch,
  ChildUnitReach,
  ChildUnitsAccess,
  ClearedAccessItem,
  GrantedChildUnit,
  OrganizationAccessDraft,
  OrganizationAccessOffers,
  OrganizationAccessScope,
  OtherTreesAccess,
  SharingPolicySavePlan,
} from './models/organization-access';
export type {
  AllUnitsSection,
  ChildUnitsSection,
  OtherTreeRootsSection,
  SharingChildUnit,
  SharingPolicy,
  SharingPolicyListResponse,
  SharingPolicyRequest,
  SharingPolicySections,
  SharingReach,
  SharingRootUnit,
} from './models/sharing-policy';

// Utils
export {default as draftToSharingSections, hasNoSharingSections} from './utils/draftToSharingSections';
export {default as getOrganizationAccessOffers} from './utils/getOrganizationAccessOffers';
export {
  canEditSharingSectionsInPlace,
  default as planSharingPolicySave,
  haveSameSharingSections,
} from './utils/planSharingPolicySave';
export {default as sharingPolicyToDraft, EMPTY_ORGANIZATION_ACCESS_DRAFT} from './utils/sharingPolicyToDraft';
export {
  default as translateOrganizationAccessWording,
  organizationAccessWordingKey,
  ORGANIZATION_ACCESS_WORDING_FALLBACKS,
  type OrganizationAccessTranslateFn,
  type OrganizationAccessWordingKey,
  type OrganizationAccessWordingValues,
} from './utils/translateOrganizationAccessWording';
export {
  default as switchOrganizationAccess,
  getBlockedNoneReason,
  type BlockedNoneReason,
} from './utils/switchOrganizationAccess';
