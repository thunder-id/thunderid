/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

/**
 * Styles for the Checkbox primitive component.
 *
 * BEM block: `.thunderid-checkbox`
 *
 * Modifiers:
 *   --error  – shows validation error state
 *
 * Elements:
 *   __wrapper | __input | __label | __error
 */
const CHECKBOX_CSS = `
/* ============================================================
   Checkbox
   ============================================================ */

.thunderid-checkbox {
  display: flex;
  flex-direction: column;
  gap: calc(var(--thunder-spacing-unit) * 0.5);
  font-family: var(--thunder-typography-fontFamily);
}

.thunderid-checkbox__wrapper {
  display: inline-flex;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 0.75);
  cursor: pointer;
  user-select: none;
}

.thunderid-checkbox__input {
  width: var(--thunder-checkbox-size);
  height: var(--thunder-checkbox-size);
  cursor: pointer;
  accent-color: var(--thunder-color-primary-main);
  flex-shrink: 0;
  border-radius: var(--thunder-border-radius-xs);
}
.thunderid-checkbox__input:focus-visible {
  outline: none;
  box-shadow: 0 0 0 var(--thunder-focus-ring-width) var(--thunder-focus-ring-color);
}
.thunderid-checkbox__input:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.thunderid-checkbox__label {
  font-size: var(--thunder-typography-fontSize-md);
  color: var(--thunder-color-text-primary);
  line-height: var(--thunder-typography-lineHeight-normal);
}

.thunderid-checkbox__error {
  font-size: var(--thunder-typography-fontSize-xs);
  color: var(--thunder-color-error-contrastText);
  line-height: var(--thunder-typography-lineHeight-normal);
}
`;

export default CHECKBOX_CSS;
