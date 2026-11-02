// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

export const FlowTypes = {
  REGISTRATION: 'REGISTRATION',
  PASSWORD_RECOVERY: 'PASSWORD_RECOVERY',
  LOGIN: 'LOGIN',
} as const;

export type FlowTypes = (typeof FlowTypes)[keyof typeof FlowTypes];

/**
 * Interface for common metadata.
 */
export interface MetadataInterface {
  /**
   * The type of the flow.
   */
  flowType: FlowTypes;
  /**
   * Supported executors for the flow.
   */
  supportedExecutors: string[];
  /**
   * Connector configuration for the flow.
   */
  connectorConfigs: ConnectorConfigs;
  /**
   * The default attribute profile to be used.
   */
  attributeProfile: string;
  /**
   * Supported flow completion configurations.
   */
  supportedFlowCompletionConfigs?: string[];
  /**
   * Metadata for attributes used in the flow.
   */
  attributeMetadata: AttributeMetadataInterface[];
  /**
   * List of executor connections.
   */
  executorConnections: ExecutorConnectionInterface[];
}

/**
 * Common connector configuration interface.
 */
export interface ConnectorConfigs {
  /**
   * Indicates if multi-attribute login is enabled.
   */
  multiAttributeLoginEnabled: boolean;
  /**
   * Indicates if account verification is enabled.
   */
  accountVerificationEnabled: boolean;
}

/**
 * Interface for attribute metadata.
 */
export interface AttributeMetadataInterface {
  /**
   * The name of the attribute.
   */
  name: string;
  /**
   * Claim URI of the attribute.
   */
  claimURI: string;
  /**
   * Indicates if the attribute is required.
   */
  required: boolean;
  /**
   * Indicates if the attribute is read-only.
   */
  readOnly: boolean;
  /**
   * List of validators for the attribute.
   */
  validators: string[];
}

/**
 * Interface for executor connection.
 */
export interface ExecutorConnectionInterface {
  /**
   * The name of the executor.
   */
  executorName: string;
  /**
   * List of connections for the executor.
   */
  connections: string[];
}

/**
 *  Captures claim management properties.
 */
export interface Claim {
  id: string;
  claimURI: string;
  dialectURI?: string;
  description: string;
  displayOrder: number;
  multiValued: boolean;
  dataType: string;
  subAttributes?: string[];
  canonicalValues?: unknown[];
  displayName: string;
  readOnly: boolean;
  regEx: string;
  required: boolean;
  supportedByDefault: boolean;
}
