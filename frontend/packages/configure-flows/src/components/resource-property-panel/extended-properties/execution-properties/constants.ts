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
    value: 'group_member_removal',
    translationKey: 'flows:core.executions.accessChangeValidator.mode.groupMemberRemoval',
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
    value: 'remove_assignment',
    translationKey: 'flows:core.executions.roleExecutor.mode.removeAssignment',
    displayLabel: 'Remove Role Assignment',
  },
  {value: 'delete', translationKey: 'flows:core.executions.roleExecutor.mode.delete', displayLabel: 'Delete Role'},
  {
    value: 'remove_permissions',
    translationKey: 'flows:core.executions.roleExecutor.mode.removePermissions',
    displayLabel: 'Change Role Permissions',
  },
] as const;

/**
 * Available modes for the Group executor. Must match its SupportedModes in the backend.
 */
export const GROUP_EXECUTOR_MODES = [
  {value: 'delete', translationKey: 'flows:core.executions.groupExecutor.mode.delete', displayLabel: 'Delete Group'},
  {
    value: 'remove_member',
    translationKey: 'flows:core.executions.groupExecutor.mode.removeMember',
    displayLabel: 'Remove Group Member',
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
 * Entity categories a mode-driven executor can work on. The value is the category name the backend
 * reads from the node's `mode` property.
 */
export const ENTITY_MODE_OPTIONS = [
  {value: 'user', translationKey: 'flows:core.executions.entityMode.user'},
  {value: 'agent', translationKey: 'flows:core.executions.entityMode.agent'},
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
  ExecutionTypes.AccessChangeValidator,
  ExecutionTypes.RoleExecutor,
  ExecutionTypes.GroupExecutor,
]);
