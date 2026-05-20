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
 * Styles for the OrganizationList presentation component.
 *
 * BEM block: `.thunderid-organization-list`
 *
 * The root element is a plain `div`. There is no Card wrapper here,
 * so this file provides the full layout including border and spacing.
 *
 * Elements:
 *   __loading  – loading state container (centred Spinner)
 *   __empty    – empty state message (Typography body2)
 *   __item     – each selectable organization row button
 */
const ORGANIZATION_LIST_CSS = `
/* ============================================================
   OrganizationList
   ============================================================ */

.thunderid-organization-list {
  display: flex;
  flex-direction: column;
  gap: calc(var(--thunder-spacing-unit) * 0.5);
  font-family: var(--thunder-typography-fontFamily);
}

/* Loading / Empty ------------------------------------------- */

.thunderid-organization-list__loading,
.thunderid-organization-list__empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: calc(var(--thunder-spacing-unit) * 3);
  color: var(--thunder-color-text-secondary);
}

/* Items ----------------------------------------------------- */

.thunderid-organization-list__item {
  display: flex;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 1.25);
  width: 100%;
  padding: calc(var(--thunder-spacing-unit) * 1.25) calc(var(--thunder-spacing-unit) * 1.5);
  background: var(--thunder-color-background-surface);
  border: 1px solid var(--thunder-color-border);
  border-radius: var(--thunder-border-radius-small);
  cursor: pointer;
  text-align: left;
  font-family: var(--thunder-typography-fontFamily);
  font-size: var(--thunder-typography-fontSize-md);
  color: var(--thunder-color-text-primary);
  transition:
    background-color var(--thunder-transition-fast),
    border-color var(--thunder-transition-fast),
    box-shadow var(--thunder-transition-fast);
  box-sizing: border-box;
}

.thunderid-organization-list__item:hover {
  background-color: var(--thunder-color-primary-light);
  border-color: var(--thunder-color-primary-main);
}

.thunderid-organization-list__item:focus-visible {
  outline: none;
  box-shadow: 0 0 0 var(--thunder-focus-ring-width) var(--thunder-focus-ring-color);
}
`;

export default ORGANIZATION_LIST_CSS;
