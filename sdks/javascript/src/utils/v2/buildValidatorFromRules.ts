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

import evaluateValidationRule from './evaluateValidationRule';
import {ValidationRule} from '../../models/v2/embedded-flow-v2';

/**
 * Composes an array of `ValidationRule`s into a single validator function suitable for
 * `useForm`'s `FormField.validator` slot.
 *
 * The composed validator evaluates rules in declaration order and returns the **first**
 * failing rule's message — matching the SDK's render-prop shape of a single string per
 * field. When all rules pass it returns `null`.
 *
 * Returns `null` when no rules are supplied so callers can compose conditionally.
 */
const buildValidatorFromRules = (
  rules: ValidationRule[] | undefined,
): ((value: string) => string | null) | null => {
  if (!rules || rules.length === 0) {
    return null;
  }
  return (value: string): string | null => {
    for (const rule of rules) {
      const message: string | null = evaluateValidationRule(rule, value);
      if (message !== null) {
        return message;
      }
    }
    return null;
  };
};

export default buildValidatorFromRules;
