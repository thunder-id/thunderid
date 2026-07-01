/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/** A selectively disclosable claim mapped from the user's profile attribute. */
export interface ClaimMapping {
  name: string;
  displayName?: string;
}

/** Wallet-facing display metadata with no admin-facing equivalent (name/description come from the config itself). */
export interface CredentialDisplay {
  locale?: string;
  logoUri?: string;
}

/**
 * An OpenID4VCI credential configuration managed in the console. The handle is
 * the credential_configuration_id and the OAuth scope.
 */
export interface VerifiableCredential {
  id: string;
  handle: string;
  ouId: string;
  ouHandle?: string;
  name?: string;
  description?: string;
  format: string;
  vct: string;
  claims?: ClaimMapping[];
  display?: CredentialDisplay;
  validitySeconds?: number;
}

/**
 * Minimal projection returned by the list endpoint — only the fields the
 * management table renders. Use VerifiableCredential for the detail view.
 */
export interface VerifiableCredentialSummary {
  id: string;
  handle: string;
  ouId: string;
  ouHandle?: string;
  format: string;
  vct: string;
  name?: string;
}

/** The list endpoint returns a plain array of credential configuration summaries. */
export type VCListResponse = VerifiableCredentialSummary[];

/** Response of the issuer-initiated credential offer endpoint. */
export interface CredentialOfferResponse {
  credential_offer: Record<string, unknown>;
  credential_offer_uri: string;
}
