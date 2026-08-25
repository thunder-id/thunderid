// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import type {Base} from './base';
import type {Element} from './elements';

export interface StepPosition {
  x: number;
  y: number;
}

export interface StepSize {
  width: number;
  height: number;
}

export interface StrictStep extends Base {
  size: StepSize;
  position: StepPosition;
}

export interface StepWithCodeGenerationMetadata extends StrictStep {
  __generationMeta__: unknown;
}

export interface StepAction {
  /**
   * The ID of the next step to navigate to on success.
   */
  onSuccess?: string;
  /**
   * The ID of the next step to navigate to on failure.
   * Used for TASK_EXECUTION nodes that support branching.
   */
  onFailure?: string;
  /**
   * The ID of the next step to navigate to on incomplete.
   * Used for TASK_EXECUTION nodes.
   */
  onIncomplete?: string;
  /**
   * The executor configuration for this action.
   */
  executor?: {
    name?: string;
    [key: string]: unknown;
  };
  [key: string]: unknown;
}

export interface StepData {
  components?: Element[];
  action?: StepAction;
  /**
   * Node-specific properties persisted in the flow definition
   * (e.g. a user-set `displayName`, executor options).
   */
  properties?: Record<string, unknown>;
  [key: string]: unknown;
}

export type Step = StepWithCodeGenerationMetadata & Node<StepData>;

export const StepCategories = {
  Decision: 'DECISION',
  Interface: 'INTERFACE',
  Workflow: 'WORKFLOW',
  Executor: 'EXECUTOR',
} as const;

export const StepTypes = {
  View: 'VIEW',
  Rule: 'RULE',
  Execution: 'TASK_EXECUTION',
  End: 'END',
  Call: 'CALL',
} as const;

export const StaticStepTypes = {
  UserOnboard: 'USER_ONBOARD',
  Start: 'START',
} as const;

export const ExecutionTypes = {
  GoogleFederation: 'GoogleOIDCAuthExecutor',
  GithubFederation: 'GithubOAuthExecutor',
  OpenID4VPVerify: 'OpenID4VPVerifyExecutor',
  OAuthExecutor: 'OAuthExecutor',
  OIDCAuthExecutor: 'OIDCAuthExecutor',
  PasskeyAuth: 'PasskeyAuthExecutor',
  MagicLinkExecutor: 'MagicLinkExecutor',
  OTPExecutor: 'OTPExecutor',
  ConsentExecutor: 'ConsentExecutor',
  IdentifyingExecutor: 'IdentifyingExecutor',
  OUResolverExecutor: 'OUResolverExecutor',
  InviteExecutor: 'InviteExecutor',
  EmailExecutor: 'EmailExecutor',
  SMSExecutor: 'SMSExecutor',
  CredentialSetter: 'CredentialSetter',
  AttributeUniquenessValidator: 'AttributeUniquenessValidator',
  PermissionValidator: 'PermissionValidator',
  ProvisioningExecutor: 'ProvisioningExecutor',
  HTTPRequestExecutor: 'HTTPRequestExecutor',
  OUExecutor: 'OUExecutor',
  UserTypeResolver: 'UserTypeResolver',
  SSOCheck: 'SSOCheckExecutor',
  Session: 'SessionExecutor',
  SessionSignOut: 'SessionSignOutExecutor',
  PreDelete: 'PreDeleteExecutor',
  ValidateApplicationDeletion: 'ValidateApplicationDeletionExecutor',
  ValidateSecretRegeneration: 'ValidateSecretRegenerationExecutor',
  ApplicationDelete: 'ApplicationDeleteExecutor',
  ClientSecret: 'ClientSecretExecutor',
  AuthAssert: 'AuthAssertExecutor',
  Authorization: 'AuthorizationExecutor',
} as const;

export const ExecutionStepViewTypes = {
  Default: 'Execution',
  MagicLinkView: 'Magic Link View',
  PasskeyView: 'Passkey View',
};

export type StepCategories = (typeof StepCategories)[keyof typeof StepCategories];
export type StepTypes = (typeof StepTypes)[keyof typeof StepTypes];
export type StaticStepTypes = (typeof StaticStepTypes)[keyof typeof StaticStepTypes];
export type ExecutionTypes = (typeof ExecutionTypes)[keyof typeof ExecutionTypes];
export type ExecutionStepViewTypes = (typeof ExecutionStepViewTypes)[keyof typeof ExecutionStepViewTypes];

/**
 * Edge style types for the flow canvas.
 * These are used by the BaseEdge component to determine the visual style
 * while maintaining collision avoidance.
 * - default: Bézier curve (smooth curved edges) - ReactFlow's default edge type
 * - smoothstep: Smooth step edges (rounded corners)
 * - step: Step edges (right angles)
 */
export const EdgeStyleTypes = {
  Bezier: 'default',
  SmoothStep: 'smoothstep',
  Step: 'step',
} as const;

export type EdgeStyleTypes = (typeof EdgeStyleTypes)[keyof typeof EdgeStyleTypes];
