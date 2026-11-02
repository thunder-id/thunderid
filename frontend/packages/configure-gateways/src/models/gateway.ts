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
