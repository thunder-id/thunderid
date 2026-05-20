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
 * Styles for the Select primitive component.
 *
 * BEM block: `.thunderid-select`
 *
 * Modifiers:
 *   --error  – shows validation error state
 *
 * Elements:
 *   __label | __required | __input | __error | __helper
 */
const SELECT_CSS = `
/* ============================================================
   Select
   ============================================================ */

.thunderid-select {
  display: flex;
  flex-direction: column;
  gap: calc(var(--thunder-spacing-unit) * 0.5);
  font-family: var(--thunder-typography-fontFamily);
  width: 100%;
  box-sizing: border-box;
}

.thunderid-select__label {
  font-size: var(--thunder-typography-fontSize-sm);
  font-weight: var(--thunder-typography-fontWeight-medium);
  color: var(--thunder-color-text-primary);
  display: block;
  line-height: var(--thunder-typography-lineHeight-normal);
}

.thunderid-select__required {
  color: var(--thunder-color-error-main);
  margin-left: 2px;
}

.thunderid-select__input {
  width: 100%;
  height: var(--thunder-input-height);
  padding: 0 calc(var(--thunder-spacing-unit) * 4) 0 var(--thunder-input-paddingX);
  border: 1px solid var(--thunder-input-borderColor);
  border-radius: var(--thunder-input-borderRadius);
  font-family: var(--thunder-typography-fontFamily);
  font-size: var(--thunder-input-fontSize);
  color: var(--thunder-color-text-primary);
  background-color: var(--thunder-color-background-surface);
  appearance: none;
  -webkit-appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='%236b7280' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right var(--thunder-input-paddingX) center;
  cursor: pointer;
  box-sizing: border-box;
  transition:
    border-color var(--thunder-transition-fast),
    box-shadow var(--thunder-transition-fast);
  outline: none;
  line-height: var(--thunder-typography-lineHeight-normal);
}
.thunderid-select__input:focus {
  border-color: var(--thunder-input-focusBorderColor);
  box-shadow: var(--thunder-input-focusRing);
}
.thunderid-select__input:disabled {
  background-color: var(--thunder-color-background-disabled);
  color: var(--thunder-color-action-disabled);
  cursor: not-allowed;
}

.thunderid-select--error .thunderid-select__input {
  border-color: var(--thunder-color-error-main);
}
.thunderid-select--error .thunderid-select__input:focus {
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.15);
}

.thunderid-select__error {
  font-size: var(--thunder-typography-fontSize-xs);
  color: var(--thunder-color-error-contrastText);
  line-height: var(--thunder-typography-lineHeight-normal);
}

.thunderid-select__helper {
  font-size: var(--thunder-typography-fontSize-xs);
  color: var(--thunder-color-text-secondary);
  line-height: var(--thunder-typography-lineHeight-normal);
}
`;

export default SELECT_CSS;
