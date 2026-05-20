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
 * Styles for the Divider primitive component.
 *
 * BEM block: `.thunderid-divider`
 *
 * Modifiers:
 *   --horizontal   – full-width horizontal rule
 *   --vertical     – inline vertical bar
 *   --with-content – flex row with centred label between two lines
 *
 * Elements:
 *   __line | __content
 */
const DIVIDER_CSS = `
/* ============================================================
   Divider
   ============================================================ */

.thunderid-divider {
  box-sizing: border-box;
}

.thunderid-divider--horizontal {
  width: 100%;
  border: none;
  border-top: 1px solid var(--thunder-color-border);
  margin: calc(var(--thunder-spacing-unit) * 1) 0;
}

.thunderid-divider--vertical {
  display: inline-block;
  width: 1px;
  height: 100%;
  min-height: 1em;
  border: none;
  background-color: var(--thunder-color-border);
  margin: 0 calc(var(--thunder-spacing-unit) * 1);
  align-self: stretch;
}

.thunderid-divider--with-content {
  display: flex;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 1);
  border: none;
  margin: calc(var(--thunder-spacing-unit) * 1) 0;
}

.thunderid-divider__line {
  flex: 1;
  height: 1px;
  background-color: var(--thunder-color-border);
}

.thunderid-divider__content {
  flex-shrink: 0;
  font-size: var(--thunder-typography-fontSize-xs);
  color: var(--thunder-color-text-secondary);
  padding: 0 calc(var(--thunder-spacing-unit) * 0.5);
  font-family: var(--thunder-typography-fontFamily);
  text-transform: uppercase;
  letter-spacing: var(--thunder-typography-letterSpacing-wide);
  font-weight: var(--thunder-typography-fontWeight-medium);
}
`;

export default DIVIDER_CSS;
