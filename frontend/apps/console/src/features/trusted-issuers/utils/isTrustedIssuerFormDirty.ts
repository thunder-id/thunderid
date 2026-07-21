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

/** Fields compared to detect unsaved changes, in the order they appear on the form. */
const TRUSTED_ISSUER_FORM_FIELDS: (keyof TrustedIssuerFormData)[] = [
  'name',
  'issuer',
  'jwksEndpoint',
  'idJagEnabled',
  'tokenExchangeEnabled',
  'trustedTokenAudience',
];

/** Fields where the form maps an empty string to `undefined`, so the two should compare equal. */
const EMPTY_STRING_AS_UNDEFINED_FIELDS: (keyof TrustedIssuerFormData)[] = ['trustedTokenAudience'];

function normalize(field: keyof TrustedIssuerFormData, value: unknown): unknown {
  if (EMPTY_STRING_AS_UNDEFINED_FIELDS.includes(field) && value === '') {
    return undefined;
  }
  return value;
}

/**
 * Whether `values` differs from `baseline` on any known trusted-issuer form field. Compares
 * fields individually (rather than via `JSON.stringify`) so that key order and `undefined` vs.
 * missing keys don't produce false positives.
 */
export default function isTrustedIssuerFormDirty(
  values: TrustedIssuerFormData,
  baseline: TrustedIssuerFormData,
): boolean {
  return TRUSTED_ISSUER_FORM_FIELDS.some(
    (field) => normalize(field, values[field]) !== normalize(field, baseline[field]),
  );
}
