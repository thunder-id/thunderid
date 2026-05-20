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
 * Styles for the OrganizationProfile presentation component.
 *
 * BEM block: `.thunderid-organization-profile`
 */
const ORGANIZATION_PROFILE_CSS = `
/* ============================================================
   OrganizationProfile
   ============================================================ */

.thunderid-organization-profile {
  display: flex;
  flex-direction: column;
  min-width: 320px;
  padding: 0;
  overflow: hidden;
}

/* Header: title + divider ------------------------------------ */

.thunderid-organization-profile__header {
  padding: calc(var(--thunder-spacing-unit) * 2) calc(var(--thunder-spacing-unit) * 2.5);
  padding-bottom: calc(var(--thunder-spacing-unit) * 1.5);
}

.thunderid-organization-profile__title {
  margin: 0;
}

.thunderid-organization-profile__header-divider {
  margin: 0;
}

/* Identity: avatar + org name + handle ----------------------- */

.thunderid-organization-profile__identity {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: calc(var(--thunder-spacing-unit) * 2) 0 calc(var(--thunder-spacing-unit) * 1.5);
  gap: calc(var(--thunder-spacing-unit) * 0.5);
}

.thunderid-organization-profile__avatar {
  width: var(--thunder-avatar-size);
  height: var(--thunder-avatar-size);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-bottom: calc(var(--thunder-spacing-unit) * 0.375);
}

.thunderid-organization-profile__avatar-initials {
  color: #ffffff;
  font-size: var(--thunder-avatar-fontSize);
  font-weight: 600;
  line-height: 1;
  letter-spacing: 0.02em;
  pointer-events: none;
  user-select: none;
}

.thunderid-organization-profile__org-name {
  margin: 0;
  text-align: center;
}

.thunderid-organization-profile__org-handle {
  color: var(--thunder-color-text-secondary);
  text-align: center;
}

.thunderid-organization-profile__identity-divider {
  margin: 0;
}

/* Fields ---------------------------------------------------- */

.thunderid-organization-profile__fields {
  display: flex;
  flex-direction: column;
}

.thunderid-organization-profile__field {
  display: grid;
  grid-template-columns: 36% 64%;
  align-items: center;
  padding: calc(var(--thunder-spacing-unit) * 1.25) calc(var(--thunder-spacing-unit) * 2.5);
  gap: calc(var(--thunder-spacing-unit) * 0.75);
  box-sizing: border-box;
  transition: background-color var(--thunder-transition-fast);
}

.thunderid-organization-profile__field:hover {
  background-color: var(--thunder-color-action-hover);
}

.thunderid-organization-profile__field + .thunderid-organization-profile__field {
  border-top: 1px solid var(--thunder-color-border);
}

.thunderid-organization-profile__field-label-col {
  /* label column */
}

.thunderid-organization-profile__field-label {
  color: var(--thunder-color-text-secondary);
  font-size: var(--thunder-typography-fontSize-sm);
}

.thunderid-organization-profile__field-value-col {
  /* value column */
}

.thunderid-organization-profile__field-display {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: calc(var(--thunder-spacing-unit) * 0.5);
  min-height: 1.5rem;
}

.thunderid-organization-profile__field-value {
  color: var(--thunder-color-text-primary);
  word-break: break-word;
  flex: 1;
  font-size: var(--thunder-typography-fontSize-sm);
}

.thunderid-organization-profile__field-value--id {
  font-size: calc(var(--thunder-typography-fontSize-sm) * 0.9);
  color: var(--thunder-color-text-secondary);
  font-family: monospace;
  word-break: break-all;
}

.thunderid-organization-profile__field-placeholder {
  color: var(--thunder-color-primary-main);
  font-style: italic;
  font-size: var(--thunder-typography-fontSize-sm);
  flex: 1;
  cursor: pointer;
  text-decoration: underline;
  text-decoration-style: dotted;
  text-underline-offset: 2px;
}

/* Edit button (pencil icon) --------------------------------- */

.thunderid-organization-profile__field-edit-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  cursor: pointer;
  color: var(--thunder-color-text-secondary);
  flex-shrink: 0;
  padding: calc(var(--thunder-spacing-unit) * 0.375);
  border-radius: var(--thunder-border-radius-small);
  transition:
    color var(--thunder-transition-fast),
    background-color var(--thunder-transition-fast),
    opacity var(--thunder-transition-fast);
  opacity: 0;
  line-height: 0;
}

.thunderid-organization-profile__field:hover .thunderid-organization-profile__field-edit-btn {
  opacity: 1;
}

.thunderid-organization-profile__field-edit-btn:hover {
  color: var(--thunder-color-primary-main);
  background-color: var(--thunder-color-primary-light);
}

.thunderid-organization-profile__field-edit-btn:focus-visible {
  opacity: 1;
  outline: none;
  box-shadow: 0 0 0 var(--thunder-focus-ring-width) var(--thunder-focus-ring-color);
}

/* Edit mode ------------------------------------------------- */

.thunderid-organization-profile__field-edit {
  display: flex;
  flex-direction: column;
  gap: calc(var(--thunder-spacing-unit) * 0.75);
}

.thunderid-organization-profile__field-edit-actions {
  display: flex;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 0.75);
}
`;

export default ORGANIZATION_PROFILE_CSS;
