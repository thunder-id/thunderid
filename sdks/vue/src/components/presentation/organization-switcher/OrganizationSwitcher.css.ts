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
 * Styles for the OrganizationSwitcher presentation component.
 *
 * BEM block: `.thunderid-organization-switcher`
 *
 * The root element is a Card (`.thunderid-card`), so we override its
 * default padding to let the trigger button fill the surface edge-to-edge,
 * and apply `position: relative` to anchor the absolute dropdown.
 *
 * Modifiers:  (none — state is controlled via isOpen in component logic)
 *
 * Elements:
 *   __trigger        – the clickable trigger button showing current org
 *   __trigger-label  – the org name Typography inside the trigger
 *   __dropdown       – the absolute-positioned dropdown listbox
 *   __loading        – loading state container (Spinner)
 *   __empty          – empty state message (Typography)
 *   __item           – each selectable organization row
 *   __item--active   – currently selected organization
 */
const ORGANIZATION_SWITCHER_CSS = `
/* ============================================================
   OrganizationSwitcher
   ============================================================ */

/* Override Card's default padding so the trigger button fills the surface */
.thunderid-organization-switcher.thunderid-card {
  padding: 0;
  position: relative;
  display: inline-block;
  min-width: 180px;
}

/* Trigger ---------------------------------------------------- */

.thunderid-organization-switcher__trigger {
  display: flex;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 0.75);
  width: 100%;
  padding: var(--thunder-dropdown-itemPaddingY) var(--thunder-dropdown-itemPaddingX);
  background: none;
  border: none;
  cursor: pointer;
  border-radius: var(--thunder-dropdown-borderRadius);
  color: var(--thunder-color-text-primary);
  font-family: var(--thunder-typography-fontFamily);
  font-size: var(--thunder-typography-fontSize-md);
  transition: background-color var(--thunder-transition-fast);
  text-align: left;
  box-sizing: border-box;
}

.thunderid-organization-switcher__trigger:hover {
  background-color: var(--thunder-color-action-hover);
}

.thunderid-organization-switcher__trigger:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 var(--thunder-focus-ring-width) var(--thunder-focus-ring-color);
  border-radius: var(--thunder-dropdown-borderRadius);
}

.thunderid-organization-switcher__trigger-label {
  flex: 1;
  text-align: left;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

/* Dropdown --------------------------------------------------- */

.thunderid-organization-switcher__dropdown {
  position: absolute;
  top: calc(100% + calc(var(--thunder-spacing-unit) * 0.5));
  left: 0;
  right: 0;
  z-index: 1000;
  background-color: var(--thunder-color-background-surface);
  border: 1px solid var(--thunder-color-border);
  border-radius: var(--thunder-dropdown-borderRadius);
  box-shadow: var(--thunder-dropdown-shadow);
  overflow: hidden;
  min-width: 160px;
  display: flex;
  flex-direction: column;
  padding: calc(var(--thunder-spacing-unit) * 0.5) 0;
}

/* Loading / Empty states ------------------------------------ */

.thunderid-organization-switcher__loading,
.thunderid-organization-switcher__empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: calc(var(--thunder-spacing-unit) * 2);
  color: var(--thunder-color-text-secondary);
}

/* Items ----------------------------------------------------- */

.thunderid-organization-switcher__item {
  display: flex;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 0.75);
  width: 100%;
  padding: var(--thunder-dropdown-itemPaddingY) var(--thunder-dropdown-itemPaddingX);
  background: none;
  border: none;
  cursor: pointer;
  text-align: left;
  font-family: var(--thunder-typography-fontFamily);
  font-size: var(--thunder-typography-fontSize-md);
  color: var(--thunder-color-text-primary);
  transition: background-color var(--thunder-transition-fast);
  box-sizing: border-box;
}

.thunderid-organization-switcher__item:hover {
  background-color: var(--thunder-color-action-hover);
}

.thunderid-organization-switcher__item--active {
  background-color: var(--thunder-color-action-selected);
  color: var(--thunder-color-primary-main);
  font-weight: var(--thunder-typography-fontWeight-medium);
}

.thunderid-organization-switcher__item--active:hover {
  background-color: var(--thunder-color-action-focus);
}
`;

export default ORGANIZATION_SWITCHER_CSS;
