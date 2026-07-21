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

import type {TrustedIssuerFormData} from '../models/trusted-issuer';

/** The subset of trusted-issuer fields collected via text inputs. */
export type TrustedIssuerTextFields = Pick<TrustedIssuerFormData, 'name' | 'issuer' | 'jwksEndpoint'>;

/** A validation failure reason for a single field. */
export type TrustedIssuerFieldErrorKind = 'required' | 'url';

/** Field name → failure reason, present only for invalid fields. */
export type TrustedIssuerFormErrors = Partial<Record<keyof TrustedIssuerTextFields, TrustedIssuerFieldErrorKind>>;

/** Whether a value is an absolute `https://` URL. */
function isValidHttpsUrl(value: string): boolean {
  try {
    return new URL(value).protocol === 'https:';
  } catch {
    return false;
  }
}

/**
 * Validate trusted-issuer form values. Name is required; issuer and JWKS endpoint are required
 * and must be `https://` URLs. Returns a map of field name → failure reason (empty when valid).
 */
export default function validateTrustedIssuerForm(values: TrustedIssuerTextFields): TrustedIssuerFormErrors {
  const errors: TrustedIssuerFormErrors = {};

  if (!values.name.trim()) {
    errors.name = 'required';
  }

  const issuer: string = values.issuer.trim();
  if (!issuer) {
    errors.issuer = 'required';
  } else if (!isValidHttpsUrl(issuer)) {
    errors.issuer = 'url';
  }

  const jwksEndpoint: string = values.jwksEndpoint.trim();
  if (!jwksEndpoint) {
    errors.jwksEndpoint = 'required';
  } else if (!isValidHttpsUrl(jwksEndpoint)) {
    errors.jwksEndpoint = 'url';
  }

  return errors;
}
