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
 * Styles for the OtpField primitive component.
 *
 * BEM block: `.thunderid-otp-field`
 *
 * Elements:
 *   __label | __required | __inputs | __digit | __error
 */
const OTP_FIELD_CSS = `
/* ============================================================
   OtpField
   ============================================================ */

.thunderid-otp-field {
  display: flex;
  flex-direction: column;
  gap: calc(var(--thunder-spacing-unit) * 0.75);
  font-family: var(--thunder-typography-fontFamily);
}

.thunderid-otp-field__label {
  font-size: var(--thunder-typography-fontSize-sm);
  font-weight: var(--thunder-typography-fontWeight-medium);
  color: var(--thunder-color-text-primary);
  display: block;
  line-height: var(--thunder-typography-lineHeight-normal);
}

.thunderid-otp-field__required {
  color: var(--thunder-color-error-main);
  margin-left: 2px;
}

.thunderid-otp-field__inputs {
  display: flex;
  gap: calc(var(--thunder-spacing-unit) * 0.75);
}

.thunderid-otp-field__digit {
  width: var(--thunder-input-height);
  height: var(--thunder-input-height);
  text-align: center;
  border: 1px solid var(--thunder-input-borderColor);
  border-radius: var(--thunder-input-borderRadius);
  font-family: var(--thunder-typography-fontFamily);
  font-size: var(--thunder-typography-fontSize-lg);
  font-weight: var(--thunder-typography-fontWeight-semibold);
  color: var(--thunder-color-text-primary);
  background-color: var(--thunder-color-background-surface);
  box-sizing: border-box;
  outline: none;
  transition:
    border-color var(--thunder-transition-fast),
    box-shadow var(--thunder-transition-fast);
}
.thunderid-otp-field__digit:focus {
  border-color: var(--thunder-input-focusBorderColor);
  box-shadow: var(--thunder-input-focusRing);
}
.thunderid-otp-field__digit:disabled {
  background-color: var(--thunder-color-background-disabled);
  color: var(--thunder-color-action-disabled);
  cursor: not-allowed;
}

.thunderid-otp-field__error {
  font-size: var(--thunder-typography-fontSize-xs);
  color: var(--thunder-color-error-contrastText);
  line-height: var(--thunder-typography-lineHeight-normal);
}
`;

export default OTP_FIELD_CSS;
