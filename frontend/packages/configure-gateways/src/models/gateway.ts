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
  /** The default gateway, whose base URL the console shows for runtime endpoints. At most one is. */
  isDefault?: boolean;
  /**
   * Whether the gateway serves. One still being provisioned is not ready: nothing is deployed to it and
   * its values cannot be read or set until it is. Absent from a control plane that does not report it,
   * which means ready.
   */
  ready?: boolean;
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
  /** Makes this the default gateway, taking that over from the one that is. */
  isDefault?: boolean;
  /** False registers a gateway still being provisioned; true when omitted. */
  ready?: boolean;
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
  /** Marks the gateway ready, as one registered while it was provisioned is once it serves. */
  ready?: boolean;
}
