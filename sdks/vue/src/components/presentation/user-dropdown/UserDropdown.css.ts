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
 * Styles for the UserDropdown presentation component.
 *
 * BEM block: `.thunderid-user-dropdown`
 *
 * Trigger modifiers:
 *   __trigger--open               – ring + border while menu is visible
 *   __avatar--sm / --lg           – trigger avatar size variants (default is 32 px)
 *
 * Menu modifiers:
 *   __menu--align-left            – panel opens to the left of the trigger
 *   __menu--size-sm               – compact menu (180 px min-width, tighter padding)
 *   __menu--size-lg               – spacious menu (280 px min-width, more padding)
 *
 * Item modifiers:
 *   __item--danger                – destructive action (red text/hover)
 *
 * Elements:
 *   __chevron                     – rotates 180° when menu is open
 *   __menu-header                 – user identity section at top of menu
 *   __menu-header-avatar          – gradient avatar circle in header
 *   __menu-header-info            – name + subtitle column
 *   __menu-header-name            – bold display name
 *   __menu-header-subtitle        – muted email / username
 *   __menu-divider                – thin horizontal separator
 */
const USER_DROPDOWN_CSS = `
/* ============================================================
   UserDropdown
   ============================================================ */

.thunderid-user-dropdown {
  position: relative;
  display: inline-block;
  font-family: var(--thunder-typography-fontFamily);
}

/* ── Trigger ─────────────────────────────────────────────────── */

.thunderid-user-dropdown__trigger {
  display: inline-flex;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 0.5);
  padding: 3px;
  background: none;
  border: 2px solid transparent;
  border-radius: var(--thunder-border-radius-full);
  cursor: pointer;
  color: var(--thunder-color-text-primary);
  transition:
    border-color var(--thunder-transition-fast),
    box-shadow var(--thunder-transition-fast);
  box-sizing: border-box;
  outline: none;
}

.thunderid-user-dropdown__trigger:hover {
  border-color: var(--thunder-color-primary-main);
}

.thunderid-user-dropdown__trigger--open {
  border-color: var(--thunder-color-primary-main);
  box-shadow: 0 0 0 3px var(--thunder-focus-ring-color);
}

.thunderid-user-dropdown__trigger:focus-visible {
  border-color: var(--thunder-color-primary-main);
  box-shadow: 0 0 0 var(--thunder-focus-ring-width) var(--thunder-focus-ring-color);
}

/* ── Trigger avatar ──────────────────────────────────────────── */

.thunderid-user-dropdown__avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  color: #ffffff;
  flex-shrink: 0;
  font-size: var(--thunder-typography-fontSize-sm);
  font-weight: var(--thunder-typography-fontWeight-semibold);
  line-height: 1;
  user-select: none;
  pointer-events: none;
}

/* sm — 28 px */
.thunderid-user-dropdown__avatar--sm {
  width: 28px;
  height: 28px;
  font-size: var(--thunder-typography-fontSize-xs);
}

/* lg — 38 px */
.thunderid-user-dropdown__avatar--lg {
  width: 38px;
  height: 38px;
  font-size: var(--thunder-typography-fontSize-md);
}

/* ── Chevron ─────────────────────────────────────────────────── */

.thunderid-user-dropdown__chevron {
  display: inline-flex;
  align-items: center;
  color: var(--thunder-color-text-secondary);
  transition: transform var(--thunder-transition-normal);
  padding-right: calc(var(--thunder-spacing-unit) * 0.25);
}

.thunderid-user-dropdown__trigger--open .thunderid-user-dropdown__chevron {
  transform: rotate(180deg);
}

/* ── Dropdown menu ───────────────────────────────────────────── */

.thunderid-user-dropdown__menu {
  position: absolute;
  top: calc(100% + calc(var(--thunder-spacing-unit) * 0.75));
  right: 0;
  z-index: 1000;
  background-color: var(--thunder-color-background-surface);
  border: 1px solid var(--thunder-color-border);
  border-radius: var(--thunder-dropdown-borderRadius);
  box-shadow: var(--thunder-dropdown-shadow);
  overflow: hidden;
  min-width: 220px;
  display: flex;
  flex-direction: column;
  animation: thunderid-dropdown-enter var(--thunder-transition-fast) ease;
}

/* Alignment */

.thunderid-user-dropdown__menu--align-left {
  right: auto;
  left: 0;
}

/* Size: sm */

.thunderid-user-dropdown__menu--size-sm {
  min-width: 180px;
}

.thunderid-user-dropdown__menu--size-sm .thunderid-user-dropdown__menu-header {
  padding: calc(var(--thunder-spacing-unit) * 1.25) calc(var(--thunder-spacing-unit) * 1.5);
  gap: calc(var(--thunder-spacing-unit) * 1);
}

.thunderid-user-dropdown__menu--size-sm .thunderid-user-dropdown__menu-header-avatar {
  width: 30px;
  height: 30px;
  font-size: var(--thunder-typography-fontSize-sm);
}

.thunderid-user-dropdown__menu--size-sm .thunderid-user-dropdown__item {
  padding: calc(var(--thunder-spacing-unit) * 0.75) calc(var(--thunder-spacing-unit) * 1.5);
  font-size: var(--thunder-typography-fontSize-xs);
}

/* Size: lg */

.thunderid-user-dropdown__menu--size-lg {
  min-width: 280px;
}

.thunderid-user-dropdown__menu--size-lg .thunderid-user-dropdown__menu-header {
  padding: calc(var(--thunder-spacing-unit) * 2) calc(var(--thunder-spacing-unit) * 2);
  gap: calc(var(--thunder-spacing-unit) * 1.5);
}

.thunderid-user-dropdown__menu--size-lg .thunderid-user-dropdown__menu-header-avatar {
  width: 42px;
  height: 42px;
  font-size: var(--thunder-typography-fontSize-lg);
}

.thunderid-user-dropdown__menu--size-lg .thunderid-user-dropdown__item {
  padding: calc(var(--thunder-spacing-unit) * 1.25) calc(var(--thunder-spacing-unit) * 2);
  font-size: var(--thunder-typography-fontSize-md);
}

@keyframes thunderid-dropdown-enter {
  from {
    opacity: 0;
    transform: translateY(-6px) scale(0.97);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

/* ── Menu header (user identity) ─────────────────────────────── */

.thunderid-user-dropdown__menu-header {
  display: flex;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 1.25);
  padding: calc(var(--thunder-spacing-unit) * 1.5) calc(var(--thunder-spacing-unit) * 1.75);
}

.thunderid-user-dropdown__menu-header-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  color: #ffffff;
  flex-shrink: 0;
  font-size: var(--thunder-typography-fontSize-md);
  font-weight: var(--thunder-typography-fontWeight-semibold);
  line-height: 1;
  user-select: none;
}

.thunderid-user-dropdown__menu-header-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.thunderid-user-dropdown__menu-header-name {
  font-size: var(--thunder-typography-fontSize-sm);
  font-weight: var(--thunder-typography-fontWeight-semibold);
  color: var(--thunder-color-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: var(--thunder-typography-lineHeight-tight);
}

.thunderid-user-dropdown__menu-header-subtitle {
  font-size: var(--thunder-typography-fontSize-xs);
  color: var(--thunder-color-text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: var(--thunder-typography-lineHeight-normal);
}

/* ── Menu divider ────────────────────────────────────────────── */

.thunderid-user-dropdown__menu-divider {
  height: 1px;
  background-color: var(--thunder-color-border);
  margin: calc(var(--thunder-spacing-unit) * 0.5) 0;
  flex-shrink: 0;
}

/* ── Menu items ──────────────────────────────────────────────── */

.thunderid-user-dropdown__item {
  display: flex;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 1);
  width: 100%;
  padding: calc(var(--thunder-spacing-unit) * 1) calc(var(--thunder-spacing-unit) * 1.75);
  background: none;
  border: none;
  cursor: pointer;
  text-align: left;
  font-family: var(--thunder-typography-fontFamily);
  font-size: var(--thunder-typography-fontSize-sm);
  color: var(--thunder-color-text-primary);
  transition: background-color var(--thunder-transition-fast);
  box-sizing: border-box;
}

.thunderid-user-dropdown__item:hover {
  background-color: var(--thunder-color-action-hover);
}

.thunderid-user-dropdown__item:focus-visible {
  outline: none;
  background-color: var(--thunder-color-action-focus);
}

/* Danger variant (sign-out) */

.thunderid-user-dropdown__item--danger {
  color: var(--thunder-color-error-main);
}

.thunderid-user-dropdown__item--danger:hover {
  background-color: var(--thunder-color-error-light);
}

/* ── Modal overlay ───────────────────────────────────────────── */

.thunderid-user-dropdown__modal-overlay {
  position: fixed;
  inset: 0;
  background-color: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  backdrop-filter: blur(3px);
  animation: thunderid-overlay-enter var(--thunder-transition-fast) ease;
}

@keyframes thunderid-overlay-enter {
  from { opacity: 0; }
  to   { opacity: 1; }
}

/* ── Modal content ───────────────────────────────────────────── */

.thunderid-user-dropdown__modal-content {
  background: var(--thunder-color-background-surface);
  border-radius: var(--thunder-border-radius-large);
  box-shadow: var(--thunder-shadow-large);
  max-width: 480px;
  width: 92%;
  max-height: 90vh;
  overflow-y: auto;
  position: relative;
  animation: thunderid-modal-enter var(--thunder-transition-normal) ease;
}

@keyframes thunderid-modal-enter {
  from {
    opacity: 0;
    transform: translateY(12px) scale(0.97);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

/* ── Modal close button ──────────────────────────────────────── */

.thunderid-user-dropdown__modal-close {
  position: absolute;
  top: calc(var(--thunder-spacing-unit) * 1.25);
  right: calc(var(--thunder-spacing-unit) * 1.25);
  background: none;
  border: none;
  cursor: pointer;
  color: var(--thunder-color-text-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: calc(var(--thunder-spacing-unit) * 0.625);
  border-radius: var(--thunder-border-radius-small);
  z-index: 10001;
  transition:
    color var(--thunder-transition-fast),
    background-color var(--thunder-transition-fast);
  line-height: 0;
}

.thunderid-user-dropdown__modal-close:hover {
  color: var(--thunder-color-text-primary);
  background-color: var(--thunder-color-action-hover);
}

.thunderid-user-dropdown__modal-close:focus-visible {
  outline: none;
  box-shadow: 0 0 0 var(--thunder-focus-ring-width) var(--thunder-focus-ring-color);
}
`;

export default USER_DROPDOWN_CSS;
