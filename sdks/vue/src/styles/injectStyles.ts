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
 * Style injection orchestrator for the ThunderID Vue SDK.
 *
 * Each component owns its CSS in a co-located `*.css.ts` file.
 * This module assembles those CSS strings in a deterministic order and
 * injects them as a single `<style>` tag into `document.head` once per page —
 * subsequent calls are no-ops (idempotent).
 *
 * Architecture:
 *  - `styles/defaults.css.ts`   — `:root` CSS variable fallback values
 *  - `styles/animations.css.ts` — shared `@keyframes` used by multiple components
 *  - `components/primitives/{Component}/{Component}.css.ts` — per-component BEM rules (primitives)
 *  - `components/presentation/{component}/{Component}.css.ts` — composition/layout rules (presentation)
 *
 * ThemeProvider overrides CSS variables at runtime via inline styles on
 * `document.documentElement`, which wins over the `:root` stylesheet rule.
 */

import ANIMATIONS_CSS from './animations.css';
import DEFAULTS_CSS from './defaults.css';

// Primitives
import CREATE_ORGANIZATION_CSS from '../components/presentation/create-organization/CreateOrganization.css';
import LANGUAGE_SWITCHER_CSS from '../components/presentation/language-switcher/LanguageSwitcher.css';
import ORGANIZATION_LIST_CSS from '../components/presentation/organization-list/OrganizationList.css';
import ORGANIZATION_PROFILE_CSS from '../components/presentation/organization-profile/OrganizationProfile.css';
import ORGANIZATION_SWITCHER_CSS from '../components/presentation/organization-switcher/OrganizationSwitcher.css';
import USER_DROPDOWN_CSS from '../components/presentation/user-dropdown/UserDropdown.css';
import USER_PROFILE_CSS from '../components/presentation/user-profile/UserProfile.css';
import ALERT_CSS from '../components/primitives/Alert/Alert.css';
import BUTTON_CSS from '../components/primitives/Button/Button.css';
import CARD_CSS from '../components/primitives/Card/Card.css';
import CHECKBOX_CSS from '../components/primitives/Checkbox/Checkbox.css';
import DATE_PICKER_CSS from '../components/primitives/DatePicker/DatePicker.css';
import DIVIDER_CSS from '../components/primitives/Divider/Divider.css';
import LOGO_CSS from '../components/primitives/Logo/Logo.css';
import OTP_FIELD_CSS from '../components/primitives/OtpField/OtpField.css';
import PASSWORD_FIELD_CSS from '../components/primitives/PasswordField/PasswordField.css';
import SELECT_CSS from '../components/primitives/Select/Select.css';
import SPINNER_CSS from '../components/primitives/Spinner/Spinner.css';
import TEXT_FIELD_CSS from '../components/primitives/TextField/TextField.css';
import TYPOGRAPHY_CSS from '../components/primitives/Typography/Typography.css';

// Presentation

const STYLE_ID = 'thunderid-vue-styles';

/**
 * Assembled CSS for all ThunderID Vue components.
 * Order is intentional:
 *   1. CSS variable defaults + keyframes
 *   2. Primitives (lowest level, no dependencies on higher layers)
 *   3. Presentation (composed from primitives; may override primitive classes in context)
 */
const STYLES: string = [
  // Foundations
  DEFAULTS_CSS,
  ANIMATIONS_CSS,
  // Primitives
  BUTTON_CSS,
  CARD_CSS,
  TYPOGRAPHY_CSS,
  ALERT_CSS,
  TEXT_FIELD_CSS,
  PASSWORD_FIELD_CSS,
  SELECT_CSS,
  CHECKBOX_CSS,
  DATE_PICKER_CSS,
  OTP_FIELD_CSS,
  DIVIDER_CSS,
  LOGO_CSS,
  SPINNER_CSS,
  // Presentation
  ORGANIZATION_LIST_CSS,
  ORGANIZATION_SWITCHER_CSS,
  ORGANIZATION_PROFILE_CSS,
  CREATE_ORGANIZATION_CSS,
  LANGUAGE_SWITCHER_CSS,
  USER_DROPDOWN_CSS,
  USER_PROFILE_CSS,
].join('\n');

/**
 * Injects ThunderID Vue component styles into the document `<head>` once.
 * Subsequent calls are no-ops (idempotent).
 */
export function injectStyles(): void {
  if (typeof document === 'undefined') return;
  if (document.getElementById(STYLE_ID)) return;

  const style: HTMLStyleElement = document.createElement('style');
  style.id = STYLE_ID;
  style.textContent = STYLES;
  document.head.appendChild(style);
}

export default injectStyles;
