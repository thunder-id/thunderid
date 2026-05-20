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
 * Styles for the DatePicker primitive component.
 *
 * BEM block: `.thunderid-date-picker`
 *
 * Modifiers:
 *   --error  – shows validation error state
 *
 * Elements:
 *   __label | __required | __input | __error
 */
const DATE_PICKER_CSS = `
/* ============================================================
   DatePicker
   ============================================================ */

.thunderid-date-picker {
  display: flex;
  flex-direction: column;
  gap: calc(var(--thunder-spacing-unit) * 0.5);
  font-family: var(--thunder-typography-fontFamily);
  width: 100%;
  box-sizing: border-box;
}

.thunderid-date-picker__label {
  font-size: var(--thunder-typography-fontSize-sm);
  font-weight: var(--thunder-typography-fontWeight-medium);
  color: var(--thunder-color-text-primary);
  display: block;
  line-height: var(--thunder-typography-lineHeight-normal);
}

.thunderid-date-picker__required {
  color: var(--thunder-color-error-main);
  margin-left: 2px;
}

.thunderid-date-picker__input {
  width: 100%;
  height: var(--thunder-input-height);
  padding: 0 var(--thunder-input-paddingX);
  border: 1px solid var(--thunder-input-borderColor);
  border-radius: var(--thunder-input-borderRadius);
  font-family: var(--thunder-typography-fontFamily);
  font-size: var(--thunder-input-fontSize);
  color: var(--thunder-color-text-primary);
  background-color: var(--thunder-color-background-surface);
  box-sizing: border-box;
  transition:
    border-color var(--thunder-transition-fast),
    box-shadow var(--thunder-transition-fast);
  outline: none;
  cursor: pointer;
}
.thunderid-date-picker__input:focus {
  border-color: var(--thunder-input-focusBorderColor);
  box-shadow: var(--thunder-input-focusRing);
}
.thunderid-date-picker--error .thunderid-date-picker__input {
  border-color: var(--thunder-color-error-main);
}
.thunderid-date-picker--error .thunderid-date-picker__input:focus {
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.15);
}
.thunderid-date-picker__input:disabled {
  background-color: var(--thunder-color-background-disabled);
  color: var(--thunder-color-action-disabled);
  cursor: not-allowed;
}

.thunderid-date-picker__error {
  font-size: var(--thunder-typography-fontSize-xs);
  color: var(--thunder-color-error-contrastText);
  line-height: var(--thunder-typography-lineHeight-normal);
}
`;

export default DATE_PICKER_CSS;
