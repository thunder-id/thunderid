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
 * Styles for the PasswordField primitive component.
 *
 * BEM block: `.thunderid-password-field`
 *
 * Modifiers:
 *   --error  – shows validation error state
 *
 * Elements:
 *   __label | __required | __wrapper | __input | __toggle | __error
 */
const PASSWORD_FIELD_CSS = `
/* ============================================================
   PasswordField
   ============================================================ */

.thunderid-password-field {
  display: flex;
  flex-direction: column;
  gap: calc(var(--thunder-spacing-unit) * 0.5);
  font-family: var(--thunder-typography-fontFamily);
  width: 100%;
  box-sizing: border-box;
}

.thunderid-password-field__label {
  font-size: var(--thunder-typography-fontSize-sm);
  font-weight: var(--thunder-typography-fontWeight-medium);
  color: var(--thunder-color-text-primary);
  display: block;
  line-height: var(--thunder-typography-lineHeight-normal);
}

.thunderid-password-field__required {
  color: var(--thunder-color-error-main);
  margin-left: 2px;
}

.thunderid-password-field__wrapper {
  display: flex;
  align-items: center;
  height: var(--thunder-input-height);
  border: 1px solid var(--thunder-input-borderColor);
  border-radius: var(--thunder-input-borderRadius);
  background-color: var(--thunder-color-background-surface);
  transition:
    border-color var(--thunder-transition-fast),
    box-shadow var(--thunder-transition-fast);
  overflow: hidden;
  box-sizing: border-box;
}
.thunderid-password-field__wrapper:focus-within {
  border-color: var(--thunder-input-focusBorderColor);
  box-shadow: var(--thunder-input-focusRing);
}
.thunderid-password-field--error .thunderid-password-field__wrapper {
  border-color: var(--thunder-color-error-main);
}
.thunderid-password-field--error .thunderid-password-field__wrapper:focus-within {
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.15);
}

.thunderid-password-field__input {
  flex: 1;
  padding: 0 var(--thunder-input-paddingX);
  border: none;
  outline: none;
  font-family: var(--thunder-typography-fontFamily);
  font-size: var(--thunder-input-fontSize);
  color: var(--thunder-color-text-primary);
  background: transparent;
  width: 100%;
  height: 100%;
  box-sizing: border-box;
  min-width: 0;
}
.thunderid-password-field__input::placeholder {
  color: var(--thunder-color-text-secondary);
}
.thunderid-password-field__input:disabled {
  cursor: not-allowed;
}

.thunderid-password-field__toggle {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0 var(--thunder-input-paddingX);
  color: var(--thunder-color-text-secondary);
  font-size: var(--thunder-typography-fontSize-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  height: 100%;
  transition: color var(--thunder-transition-fast);
}
.thunderid-password-field__toggle:hover {
  color: var(--thunder-color-text-primary);
}

.thunderid-password-field__error {
  font-size: var(--thunder-typography-fontSize-xs);
  color: var(--thunder-color-error-contrastText);
  line-height: var(--thunder-typography-lineHeight-normal);
}
`;

export default PASSWORD_FIELD_CSS;
