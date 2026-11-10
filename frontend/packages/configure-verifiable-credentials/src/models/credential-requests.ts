// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ClaimMapping, CredentialDisplay} from './vc';

/** Request body for creating a credential configuration. */
export interface CreateVerifiableCredentialRequest {
  handle: string;
  /**
   * The organization unit this configuration belongs to. A configuration in a project leaves it out to be placed
   * in the system organization unit, or names one of the project's.
   */
  ouId?: string;
  /** The project this configuration belongs to. Required on the control plane. */
  projectId?: string;
  name?: string;
  description?: string;
  format?: string;
  vct: string;
  claims?: ClaimMapping[];
  display?: CredentialDisplay;
  validitySeconds?: number;
}

/** Request body for updating a credential configuration. */
export type UpdateVerifiableCredentialRequest = CreateVerifiableCredentialRequest;
