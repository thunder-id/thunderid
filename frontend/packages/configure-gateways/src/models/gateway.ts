// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A registered gateway. Reads never carry the key.
 *
 * @public
 */
export interface Gateway {
  id: string;
  name: string;
  baseUrl: string;
  caCertificate?: string;
  createdAt?: string;
  updatedAt?: string;
}

/**
 * The response to a registration: the gateway and the key it holds, returned only this once.
 *
 * @public
 */
export interface GatewayRegistration extends Gateway {
  key: string;
}

/**
 * @public
 */
export interface RegisterGatewayRequest {
  name: string;
  baseUrl: string;
  key?: string;
  caCertificate?: string;
}

/**
 * Every field is optional; what is omitted keeps its value. A key rotates the credential.
 *
 * @public
 */
export interface UpdateGatewayRequest {
  name?: string;
  baseUrl?: string;
  caCertificate?: string;
  key?: string;
}

/**
 * A captured configuration version. A listing leaves `resources` out.
 *
 * @public
 */
export interface ConfigurationVersion {
  version: number;
  note?: string;
  createdAt?: string;
  resources?: string;
  /** Only in a capture's answer: the resources the export could not write, left out of the version. */
  skipped?: SkippedResource[];
}

/**
 * A resource a capture left out because the export could not write it, such as a user with no username.
 *
 * @public
 */
export interface SkippedResource {
  resourceType: string;
  resourceId?: string;
  error: string;
  code?: string;
}

/**
 * @public
 */
export interface CaptureConfigurationVersionRequest {
  note?: string;
}

/**
 * The version a gateway holds and the one a revert would return it to.
 *
 * @public
 */
export interface AppliedVersion {
  gatewayId: string;
  appliedVersion?: number;
  previousVersion?: number;
  appliedAt?: string;
}

/**
 * @public
 */
export type ChangeType = 'added' | 'updated' | 'deleted' | 'unchanged';

/**
 * @public
 */
export interface ResourceChange {
  resourceType: string;
  id: string;
  name?: string;
  change: ChangeType;
}

/**
 * @public
 */
export interface DiffSummary {
  added: number;
  updated: number;
  deleted: number;
  unchanged: number;
}

/**
 * What applying a version would add, change and remove on a gateway.
 *
 * @public
 */
export interface GatewayDiff {
  fromVersion?: number;
  toVersion: number;
  summary: DiffSummary;
  changes: ResourceChange[];
}

/**
 * One resource in the gateway's account of an import.
 *
 * @public
 */
export interface ImportResourceResult {
  resourceType: string;
  resourceId?: string;
  resourceName?: string;
  operation?: string;
  status: string;
  code?: string;
  message?: string;
}

/**
 * The gateway's own account of an import.
 *
 * @public
 */
export interface ImportResult {
  summary: {
    totalDocuments: number;
    imported: number;
    deleted?: number;
    failed: number;
  };
  results?: ImportResourceResult[];
}

/**
 * @public
 */
export interface ApplyVersionRequest {
  version: string;
  dryRun: boolean;
}

/**
 * @public
 */
export interface RevertGatewayRequest {
  dryRun: boolean;
}

/**
 * The variables and secrets a version refers to that the gateway does not hold.
 *
 * @public
 */
export interface MissingValues {
  variables?: string[];
  secrets?: string[];
}

/**
 * The result of an apply or a revert. `missing` is present only when the gateway lacks values
 * the version refers to.
 *
 * @public
 */
export interface ApplyResult {
  gatewayId: string;
  dryRun: boolean;
  diff?: GatewayDiff;
  import?: ImportResult;
  missing?: MissingValues;
  recorded: boolean;
}

/**
 * @public
 */
export interface ListLink {
  href: string;
  rel: string;
}

/**
 * A variable held by a gateway.
 *
 * @public
 */
export interface GatewayVariable {
  name: string;
  value: string;
  description?: string;
  createdAt?: string;
  updatedAt?: string;
}

/**
 * A secret held by a gateway. Its value is never returned.
 *
 * @public
 */
export interface GatewaySecret {
  name: string;
  exists: boolean;
  description?: string;
  createdAt?: string;
  updatedAt?: string;
}

/**
 * @public
 */
export interface GatewayVariableList {
  totalResults: number;
  startIndex: number;
  count: number;
  variables: GatewayVariable[];
  links?: ListLink[];
}

/**
 * @public
 */
export interface GatewaySecretList {
  totalResults: number;
  startIndex: number;
  count: number;
  secrets: GatewaySecret[];
  links?: ListLink[];
}

/**
 * @public
 */
export interface GatewayValueListParams {
  limit?: number;
  offset?: number;
}

/**
 * Creates a variable or a secret on a gateway.
 *
 * @public
 */
export interface CreateGatewayValueRequest {
  name: string;
  value: string;
  description?: string;
}

/**
 * Replaces the value, and the description, of a variable or a secret on a gateway.
 *
 * @public
 */
export interface UpdateGatewayValueRequest {
  value: string;
  description?: string;
}
