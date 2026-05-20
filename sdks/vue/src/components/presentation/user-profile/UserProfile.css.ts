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
 * Styles for the UserProfile presentation component.
 *
 * BEM block: `.thunderid-user-profile`
 *
 * Modifiers:
 *   --compact   – reduced field padding for modal / dropdown embedding
 *
 * New elements in this version:
 *   __hero           – avatar + name + subtitle banner
 *   __avatar--sm/md/lg  – avatar size variants
 *   __hero-name      – prominent display name
 *   __hero-subtitle  – secondary line (email / username)
 */
const USER_PROFILE_CSS = `
/* ============================================================
   UserProfile  (modern redesign)
   ============================================================ */

.thunderid-user-profile {
  display: flex;
  flex-direction: column;
  min-width: 320px;
  overflow: hidden;
  font-family: var(--thunder-typography-fontFamily);
}

/* ── Header ─────────────────────────────────────────────────── */

.thunderid-user-profile__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: calc(var(--thunder-spacing-unit) * 2) calc(var(--thunder-spacing-unit) * 2.5)
    calc(var(--thunder-spacing-unit) * 1.75);
}

.thunderid-user-profile__title {
  margin: 0;
  font-size: var(--thunder-typography-fontSize-md);
  font-weight: var(--thunder-typography-fontWeight-semibold);
  color: var(--thunder-color-text-primary);
  letter-spacing: var(--thunder-typography-letterSpacing-tight);
}

.thunderid-user-profile__header-divider {
  margin: 0;
}

/* ── Hero (avatar + name + subtitle) ────────────────────────── */

.thunderid-user-profile__hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: calc(var(--thunder-spacing-unit) * 3) calc(var(--thunder-spacing-unit) * 2.5)
    calc(var(--thunder-spacing-unit) * 2);
  gap: calc(var(--thunder-spacing-unit) * 1.25);
  background: linear-gradient(
    180deg,
    var(--thunder-color-primary-light) 0%,
    var(--thunder-color-background-surface) 100%
  );
  border-bottom: 1px solid var(--thunder-color-border);
}

.thunderid-user-profile__avatar-wrapper {
  position: relative;
  border-radius: 50%;
  padding: 3px;
  background: linear-gradient(
    135deg,
    var(--thunder-color-primary-main),
    var(--thunder-color-primary-dark)
  );
  box-shadow: 0 4px 14px rgba(75, 110, 245, 0.28);
}

.thunderid-user-profile__avatar {
  width: var(--thunder-avatar-size, 72px);
  height: var(--thunder-avatar-size, 72px);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 2px solid var(--thunder-color-background-surface);
}

/* Avatar size variants */

.thunderid-user-profile__avatar--sm {
  width: 48px;
  height: 48px;
}

.thunderid-user-profile__avatar--sm .thunderid-user-profile__avatar-initials {
  font-size: 1rem;
}

.thunderid-user-profile__avatar--md {
  width: 64px;
  height: 64px;
}

.thunderid-user-profile__avatar--md .thunderid-user-profile__avatar-initials {
  font-size: 1.25rem;
}

.thunderid-user-profile__avatar--lg {
  width: 80px;
  height: 80px;
}

.thunderid-user-profile__avatar--lg .thunderid-user-profile__avatar-initials {
  font-size: 1.625rem;
}

.thunderid-user-profile__avatar-initials {
  color: #ffffff;
  font-weight: var(--thunder-typography-fontWeight-semibold);
  line-height: 1;
  letter-spacing: 0.02em;
  pointer-events: none;
  user-select: none;
}

.thunderid-user-profile__hero-info {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 0.375);
  text-align: center;
}

.thunderid-user-profile__hero-name {
  font-size: var(--thunder-typography-fontSize-lg);
  font-weight: var(--thunder-typography-fontWeight-semibold);
  color: var(--thunder-color-text-primary);
  line-height: var(--thunder-typography-lineHeight-tight);
  letter-spacing: var(--thunder-typography-letterSpacing-tight);
}

.thunderid-user-profile__hero-subtitle {
  font-size: var(--thunder-typography-fontSize-sm);
  color: var(--thunder-color-text-secondary);
  line-height: var(--thunder-typography-lineHeight-normal);
}

/* ── Alerts & loading ────────────────────────────────────────── */

.thunderid-user-profile__error {
  margin: calc(var(--thunder-spacing-unit) * 1.5) calc(var(--thunder-spacing-unit) * 2.5)
    calc(var(--thunder-spacing-unit) * 0.5);
}

.thunderid-user-profile__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: calc(var(--thunder-spacing-unit) * 3.5) 0;
}

/* ── Fields ──────────────────────────────────────────────────── */

.thunderid-user-profile__fields {
  display: flex;
  flex-direction: column;
}

.thunderid-user-profile__field {
  display: grid;
  grid-template-columns: 38% 62%;
  align-items: start;
  padding: calc(var(--thunder-spacing-unit) * 1.5) calc(var(--thunder-spacing-unit) * 2.5);
  gap: calc(var(--thunder-spacing-unit) * 0.75);
  box-sizing: border-box;
  transition: background-color var(--thunder-transition-fast);
}

.thunderid-user-profile__field:hover {
  background-color: var(--thunder-color-action-hover);
}

.thunderid-user-profile__field + .thunderid-user-profile__field {
  border-top: 1px solid var(--thunder-color-border);
}

.thunderid-user-profile__field-label {
  color: var(--thunder-color-text-secondary);
  font-size: var(--thunder-typography-fontSize-sm);
  font-weight: var(--thunder-typography-fontWeight-medium);
  padding-top: 2px;
}

.thunderid-user-profile__field-display {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: calc(var(--thunder-spacing-unit) * 0.5);
  min-height: 1.5rem;
}

.thunderid-user-profile__field-value {
  color: var(--thunder-color-text-primary);
  word-break: break-word;
  flex: 1;
  font-size: var(--thunder-typography-fontSize-sm);
}

.thunderid-user-profile__field-placeholder {
  color: var(--thunder-color-primary-main);
  font-size: var(--thunder-typography-fontSize-sm);
  font-style: italic;
  flex: 1;
  cursor: pointer;
  text-decoration: underline;
  text-decoration-style: dotted;
  text-underline-offset: 2px;
  opacity: 0.8;
  transition: opacity var(--thunder-transition-fast);
}

.thunderid-user-profile__field-placeholder:hover {
  opacity: 1;
}

/* ── Edit button (pencil) ────────────────────────────────────── */

.thunderid-user-profile__field-edit-btn {
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

.thunderid-user-profile__field:hover .thunderid-user-profile__field-edit-btn {
  opacity: 1;
}

.thunderid-user-profile__field-edit-btn:hover {
  color: var(--thunder-color-primary-main);
  background-color: var(--thunder-color-primary-light);
}

.thunderid-user-profile__field-edit-btn:focus-visible {
  opacity: 1;
  outline: none;
  box-shadow: 0 0 0 var(--thunder-focus-ring-width) var(--thunder-focus-ring-color);
}

/* ── Edit mode ───────────────────────────────────────────────── */

.thunderid-user-profile__field-edit {
  display: flex;
  flex-direction: column;
  gap: calc(var(--thunder-spacing-unit) * 0.75);
  padding: calc(var(--thunder-spacing-unit) * 0.25) 0;
}

.thunderid-user-profile__field-edit-actions {
  display: flex;
  align-items: center;
  gap: calc(var(--thunder-spacing-unit) * 0.75);
}

/* ── Footer slot ─────────────────────────────────────────────── */

.thunderid-user-profile__footer {
  padding: calc(var(--thunder-spacing-unit) * 1.5) calc(var(--thunder-spacing-unit) * 2.5);
  border-top: 1px solid var(--thunder-color-border);
}

/* ── Compact modifier ────────────────────────────────────────── */

.thunderid-user-profile--compact .thunderid-user-profile__hero {
  padding: calc(var(--thunder-spacing-unit) * 2) calc(var(--thunder-spacing-unit) * 2);
}

.thunderid-user-profile--compact .thunderid-user-profile__avatar--lg {
  width: 56px;
  height: 56px;
}

.thunderid-user-profile--compact .thunderid-user-profile__avatar--lg .thunderid-user-profile__avatar-initials {
  font-size: 1.125rem;
}

.thunderid-user-profile--compact .thunderid-user-profile__field {
  padding: calc(var(--thunder-spacing-unit) * 1) calc(var(--thunder-spacing-unit) * 2);
}

.thunderid-user-profile--compact .thunderid-user-profile__hero-name {
  font-size: var(--thunder-typography-fontSize-md);
}
`;

export default USER_PROFILE_CSS;
