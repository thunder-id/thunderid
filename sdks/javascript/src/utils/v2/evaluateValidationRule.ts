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

import {ValidationRule, ValidationRuleType} from '../../models/v2/embedded-flow-v2';

/**
 * Default i18n fallback keys returned when a `ValidationRule.message` is not provided.
 * Match the server-side defaults so a flow author who omits `message` sees the same
 * string regardless of whether the rule was evaluated client-side or server-side.
 */
export const DEFAULT_VALIDATION_MESSAGE_KEYS: Record<ValidationRuleType, string> = {
  regex: 'validation.pattern.invalid',
  minLength: 'validation.minLength.invalid',
  maxLength: 'validation.maxLength.invalid',
};

/**
 * Evaluates a single validation rule against the given input value.
 *
 * Returns `null` when the rule passes, or the rule's `message` (or the default
 * fallback key if `message` is absent) when it fails.
 *
 * Behavior notes:
 * - **regex**: an invalid regex pattern (one that cannot be compiled) is treated as
 *   **passing** on the client. This is lenient — the server is authoritative and
 *   will still enforce the rule if it can compile the pattern. Failing closed in the
 *   SDK risks denial-of-service for misconfigured flows.
 * - **minLength / maxLength**: compared against `value.length`. A non-numeric `value`
 *   on the rule is treated as the rule passing.
 * - Unknown rule types are treated as passing (forward compatibility with future types).
 */
const evaluateValidationRule = (rule: ValidationRule, value: string): string | null => {
  const fail = (): string => rule.message ?? DEFAULT_VALIDATION_MESSAGE_KEYS[rule.type];

  switch (rule.type) {
    case 'regex': {
      // `re.test` is unbounded; ReDoS-prone patterns are accepted here and bounded server-side.
      if (typeof rule.value !== 'string' || rule.value === '') {
        return null;
      }
      let re: RegExp;
      try {
        re = new RegExp(rule.value);
      } catch {
        return null;
      }
      return re.test(value) ? null : fail();
    }
    case 'minLength': {
      if (typeof rule.value !== 'number' || Number.isNaN(rule.value)) {
        return null;
      }
      return value.length >= rule.value ? null : fail();
    }
    case 'maxLength': {
      if (typeof rule.value !== 'number' || Number.isNaN(rule.value)) {
        return null;
      }
      return value.length <= rule.value ? null : fail();
    }
    default:
      return null;
  }
};

export default evaluateValidationRule;
