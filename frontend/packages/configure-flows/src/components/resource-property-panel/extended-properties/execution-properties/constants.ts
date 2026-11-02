// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {IdentityProviderTypes, type IdentityProviderType} from '@thunderid/configure-connections';
import {ExecutionTypes} from '../../../../models/steps';

/**
 * Maps executor names to their corresponding identity provider types.
 */
export const EXECUTOR_TO_IDP_TYPE_MAP: Record<string, IdentityProviderType> = {
  [ExecutionTypes.GoogleFederation]: IdentityProviderTypes.GOOGLE,
  [ExecutionTypes.GithubFederation]: IdentityProviderTypes.GITHUB,
  [ExecutionTypes.OAuthExecutor]: IdentityProviderTypes.OAUTH,
  [ExecutionTypes.OIDCAuthExecutor]: IdentityProviderTypes.OIDC,
};

/**
 * Set of federated executor names that support cross-OU and auth properties.
 */
export const FEDERATED_EXECUTORS = new Set<string>([
  ExecutionTypes.GoogleFederation,
  ExecutionTypes.GithubFederation,
  ExecutionTypes.OAuthExecutor,
  ExecutionTypes.OIDCAuthExecutor,
]);

/**
 * Available modes for SMS OTP executor.
 */
export const SMS_OTP_MODES = [
  {value: 'send', translationKey: 'flows:core.executions.smsOtp.mode.send', displayLabel: 'Send SMS OTP'},
  {value: 'verify', translationKey: 'flows:core.executions.smsOtp.mode.verify', displayLabel: 'Verify SMS OTP'},
] as const;

/**
 * Available modes for Magic Link executor.
 */
export const MAGIC_LINK_MODES = [
  {
    value: 'generate',
    translationKey: 'flows:core.executions.magicLink.mode.generate',
    displayLabel: 'Generate Magic Link',
  },
  {value: 'verify', translationKey: 'flows:core.executions.magicLink.mode.verify', displayLabel: 'Verify Magic Link'},
] as const;

/**
 * Available modes for Identifying executor.
 */
export const IDENTIFYING_MODES = [
  {value: 'identify', translationKey: 'flows:core.executions.identifying.mode.identify', displayLabel: 'Identify User'},
  {value: 'resolve', translationKey: 'flows:core.executions.identifying.mode.resolve', displayLabel: 'Resolve User'},
] as const;

/**
 * Available revocation modes for the Administrative Flow Pre executor.
 *
 * Must stay in step with the executor's SupportedModes in the backend
 * (backend/internal/flow/executor/administrative_flow_pre_executor.go). Offering a mode the backend
 * does not support would let the builder save a flow that fails validation.
 */
export const REVOCATION_MODES = [
  {
    value: 'revoke_all',
    translationKey: 'flows:core.executions.preDelete.mode.revokeAll',
    displayLabel: 'Validate and Plan Full Revocation',
  },
] as const;

/**
 * Available modes for the Access Change Validator executor. Must match its SupportedModes in the backend.
 */
export const ACCESS_CHANGE_VALIDATOR_MODES = [
  {
    value: 'role_assignment_removal',
    translationKey: 'flows:core.executions.accessChangeValidator.mode.roleAssignmentRemoval',
    displayLabel: 'Validate and Plan Assignment Removal',
  },
  {
    value: 'role_deletion',
    translationKey: 'flows:core.executions.accessChangeValidator.mode.roleDeletion',
    displayLabel: 'Validate and Plan Role Deletion',
  },
  {
    value: 'role_permission_removal',
    translationKey: 'flows:core.executions.accessChangeValidator.mode.rolePermissionRemoval',
    displayLabel: 'Validate and Plan Permission Change',
  },
  {
    value: 'group_deletion',
    translationKey: 'flows:core.executions.accessChangeValidator.mode.groupDeletion',
    displayLabel: 'Validate and Plan Group Deletion',
  },
  {
    value: 'group_membership_removal',
    translationKey: 'flows:core.executions.accessChangeValidator.mode.groupMembershipRemoval',
    displayLabel: 'Validate and Plan Membership Removal',
  },
  {
    value: 'action_deletion',
    translationKey: 'flows:core.executions.accessChangeValidator.mode.actionDeletion',
    displayLabel: 'Validate and Plan Action Deletion',
  },
] as const;

/**
 * Available modes for the Role executor. Must match its SupportedModes in the backend.
 */
export const ROLE_EXECUTOR_MODES = [
  {
    value: 'role_assignment_removal',
    translationKey: 'flows:core.executions.roleExecutor.mode.roleAssignmentRemoval',
    displayLabel: 'Remove Role Assignment',
  },
  {
    value: 'role_deletion',
    translationKey: 'flows:core.executions.roleExecutor.mode.roleDeletion',
    displayLabel: 'Delete Role',
  },
  {
    value: 'role_permission_removal',
    translationKey: 'flows:core.executions.roleExecutor.mode.rolePermissionRemoval',
    displayLabel: 'Change Role Permissions',
  },
] as const;

/**
 * Available modes for the Group executor. Must match its SupportedModes in the backend.
 */
export const GROUP_EXECUTOR_MODES = [
  {
    value: 'group_deletion',
    translationKey: 'flows:core.executions.groupExecutor.mode.groupDeletion',
    displayLabel: 'Delete Group',
  },
  {
    value: 'group_membership_removal',
    translationKey: 'flows:core.executions.groupExecutor.mode.groupMembershipRemoval',
    displayLabel: 'Remove Group Member',
  },
] as const;

/**
 * Available modes for the Criteria Revocation executor. Must match its SupportedModes in the backend.
 */
export const CRITERIA_REVOCATION_MODES = [
  {
    value: 'revoke',
    translationKey: 'flows:core.executions.criteriaRevocation.mode.revoke',
    displayLabel: 'Revoke Tokens',
  },
  {
    value: 'advance_cutoff',
    translationKey: 'flows:core.executions.criteriaRevocation.mode.advanceCutoff',
    displayLabel: 'Advance Revocation Cutoff',
  },
] as const;

/**
 * Identity Governance executor operations, matching its backend SupportedModes. `image` is the node icon.
 */
export const IDENTITY_GOVERNANCE_MODES = [
  {
    value: 'suspend',
    translationKey: 'flows:core.executions.identityGovernance.mode.suspend',
    displayLabel: 'Suspend Identity',
    image: 'ShieldBan',
  },
  {
    value: 'unsuspend',
    translationKey: 'flows:core.executions.identityGovernance.mode.unsuspend',
    displayLabel: 'Unsuspend Identity',
    image: 'ShieldCheck',
  },
  {
    value: 'unlock',
    translationKey: 'flows:core.executions.identityGovernance.mode.unlock',
    displayLabel: 'Unlock Identity',
    image: 'LockKeyholeOpen',
  },
] as const;

/**
 * Available modes for Passkey executor.
 */
export const PASSKEY_MODES = [
  {
    value: 'challenge',
    translationKey: 'flows:core.executions.passkey.mode.challenge',
    displayLabel: 'Request Passkey',
  },
  {value: 'verify', translationKey: 'flows:core.executions.passkey.mode.verify', displayLabel: 'Verify Passkey'},
  {
    value: 'register_start',
    translationKey: 'flows:core.executions.passkey.mode.registerStart',
    displayLabel: 'Start Passkey Registration',
  },
  {
    value: 'register_finish',
    translationKey: 'flows:core.executions.passkey.mode.registerFinish',
    displayLabel: 'Finish Passkey Registration',
  },
] as const;

/**
 * Available modes for Invite executor.
 */
export const INVITE_MODES = [
  {value: 'generate', translationKey: 'flows:core.executions.invite.mode.generate', displayLabel: 'Generate Invite'},
  {value: 'verify', translationKey: 'flows:core.executions.invite.mode.verify', displayLabel: 'Verify Invite'},
] as const;

/**
 * Available resolve strategies for OU Resolver executor.
 */
export const OU_RESOLVE_FROM_OPTIONS = [
  {value: 'caller', translationKey: 'flows:core.executions.ouResolver.resolveFrom.caller'},
  {value: 'prompt', translationKey: 'flows:core.executions.ouResolver.resolveFrom.prompt'},
  {value: 'promptAll', translationKey: 'flows:core.executions.ouResolver.resolveFrom.promptAll'},
] as const;

/**
 * Available HTTP methods for HTTP Request executor.
 */
export const HTTP_METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'] as const;

/**
 * Passkey modes that require relying party configuration.
 */
export const PASSKEY_MODES_WITH_RELYING_PARTY = ['challenge', 'register_start'] as const;

/**
 * Available input types for executor input configuration.
 * These correspond to the backend input type constants defined in common/constants.go.
 */
export const INPUT_TYPES = [
  {value: 'TEXT_INPUT', translationKey: 'flows:core.executions.inputs.types.text'},
  {value: 'EMAIL_INPUT', translationKey: 'flows:core.executions.inputs.types.email'},
  {value: 'PASSWORD_INPUT', translationKey: 'flows:core.executions.inputs.types.password'},
  {value: 'OTP_INPUT', translationKey: 'flows:core.executions.inputs.types.otp'},
  {value: 'PHONE_INPUT', translationKey: 'flows:core.executions.inputs.types.phone'},
  {value: 'CONSENT_INPUT', translationKey: 'flows:core.executions.inputs.types.consent'},
  {value: 'SELECT', translationKey: 'flows:core.executions.inputs.types.select'},
] as const;

/**
 * Executors that have fixed/programmatic inputs and should not show the input editor.
 * OAuth executors get a fixed 'code' input; ConsentExecutor gets 'consent_decisions'.
 */
export const EXECUTORS_WITH_FIXED_INPUTS = new Set<string>([
  ExecutionTypes.GoogleFederation,
  ExecutionTypes.GithubFederation,
  ExecutionTypes.OAuthExecutor,
  ExecutionTypes.OIDCAuthExecutor,
  ExecutionTypes.ConsentExecutor,
  ExecutionTypes.OpenID4VPVerify,
  ExecutionTypes.SSOCheck,
  ExecutionTypes.Session,
  ExecutionTypes.SessionSignOut,
  ExecutionTypes.PreDelete,
  ExecutionTypes.CriteriaRevocation,
  ExecutionTypes.AccessChangeValidator,
  ExecutionTypes.RoleExecutor,
  ExecutionTypes.GroupExecutor,
  ExecutionTypes.PreSuspend,
  ExecutionTypes.IdentityGovernance,
  ExecutionTypes.ExtensionExecutor,
]);
