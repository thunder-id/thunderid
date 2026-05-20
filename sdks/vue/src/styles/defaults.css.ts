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
 * Default CSS custom property fallback values.
 *
 * These are written into a `:root` rule so that every ThunderID Vue primitive
 * renders correctly even when no ThemeProvider is mounted. When ThemeProvider
 * IS present it calls `document.documentElement.style.setProperty(...)` which
 * has higher specificity than a stylesheet `:root` rule and therefore wins
 * automatically — no special cascade tricks required.
 *
 * Design token naming follows the pattern:
 *   --thunder-{category}-{sub}-{scale?}
 */
const DEFAULTS_CSS = `
/* ============================================================
   ThunderID Vue SDK – CSS variable defaults
   (ThemeProvider overrides these at runtime via inline styles)
   ============================================================ */
:root {
  /* --- Colors: Primary --- */
  --thunder-color-primary-main: #4b6ef5;
  --thunder-color-primary-light: #eef1fe;
  --thunder-color-primary-dark: #3451d1;
  --thunder-color-primary-contrastText: #ffffff;

  /* --- Colors: Secondary --- */
  --thunder-color-secondary-main: #4b5563;
  --thunder-color-secondary-light: #f3f4f6;
  --thunder-color-secondary-contrastText: #ffffff;

  /* --- Colors: Background --- */
  --thunder-color-background-surface: #ffffff;
  --thunder-color-background-body: #f9fafb;
  --thunder-color-background-disabled: #f3f4f6;
  --thunder-color-background-muted: #f1f3f5;

  /* --- Colors: Text --- */
  --thunder-color-text-primary: #111827;
  --thunder-color-text-secondary: #6b7280;

  /* --- Colors: Border --- */
  --thunder-color-border: #e5e7eb;
  --thunder-color-border-focus: var(--thunder-color-primary-main);

  /* --- Colors: Action states --- */
  --thunder-color-action-hover: rgba(0, 0, 0, 0.04);
  --thunder-color-action-selected: rgba(75, 110, 245, 0.08);
  --thunder-color-action-focus: rgba(75, 110, 245, 0.12);
  --thunder-color-action-disabled: rgba(0, 0, 0, 0.26);
  --thunder-color-action-disabledBackground: rgba(0, 0, 0, 0.08);

  /* --- Colors: Semantic --- */
  --thunder-color-error-main: #ef4444;
  --thunder-color-error-light: #fef2f2;
  --thunder-color-error-contrastText: #991b1b;
  --thunder-color-success-main: #22c55e;
  --thunder-color-success-light: #f0fdf4;
  --thunder-color-success-contrastText: #166534;
  --thunder-color-warning-main: #f59e0b;
  --thunder-color-warning-light: #fffbeb;
  --thunder-color-warning-contrastText: #92400e;
  --thunder-color-info-main: #3b82f6;
  --thunder-color-info-light: #eff6ff;
  --thunder-color-info-contrastText: #1e40af;

  /* --- Spacing --- */
  --thunder-spacing-unit: 8px;

  /* --- Border Radius --- */
  --thunder-border-radius-xs: 4px;
  --thunder-border-radius-small: 6px;
  --thunder-border-radius-medium: 10px;
  --thunder-border-radius-large: 14px;
  --thunder-border-radius-full: 9999px;

  /* --- Shadows --- */
  --thunder-shadow-xs: 0 1px 2px rgba(0, 0, 0, 0.05);
  --thunder-shadow-small: 0 1px 3px rgba(0, 0, 0, 0.08), 0 1px 2px rgba(0, 0, 0, 0.04);
  --thunder-shadow-medium: 0 4px 12px rgba(0, 0, 0, 0.08), 0 1px 3px rgba(0, 0, 0, 0.05);
  --thunder-shadow-large: 0 10px 25px rgba(0, 0, 0, 0.1), 0 2px 6px rgba(0, 0, 0, 0.05);

  /* --- Transitions --- */
  --thunder-transition-fast: 120ms ease;
  --thunder-transition-normal: 180ms ease;
  --thunder-transition-slow: 280ms ease;

  /* --- Focus Ring --- */
  --thunder-focus-ring-width: 2px;
  --thunder-focus-ring-offset: 2px;
  --thunder-focus-ring-color: rgba(75, 110, 245, 0.35);

  /* --- Typography: Font Family --- */
  --thunder-typography-fontFamily: "Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;

  /* --- Typography: Font Sizes --- */
  --thunder-typography-fontSize-xs: 0.6875rem;  /* 11px */
  --thunder-typography-fontSize-sm: 0.8125rem;  /* 13px */
  --thunder-typography-fontSize-md: 0.875rem;   /* 14px */
  --thunder-typography-fontSize-lg: 1rem;       /* 16px */
  --thunder-typography-fontSize-xl: 1.125rem;   /* 18px */
  --thunder-typography-fontSize-2xl: 1.375rem;  /* 22px */
  --thunder-typography-fontSize-3xl: 1.75rem;   /* 28px */

  /* --- Typography: Font Weights --- */
  --thunder-typography-fontWeight-normal: 400;
  --thunder-typography-fontWeight-medium: 500;
  --thunder-typography-fontWeight-semibold: 600;
  --thunder-typography-fontWeight-bold: 700;

  /* --- Typography: Line Heights --- */
  --thunder-typography-lineHeight-tight: 1.25;
  --thunder-typography-lineHeight-normal: 1.5;
  --thunder-typography-lineHeight-relaxed: 1.625;

  /* --- Typography: Letter Spacing --- */
  --thunder-typography-letterSpacing-tight: -0.01em;
  --thunder-typography-letterSpacing-normal: 0;
  --thunder-typography-letterSpacing-wide: 0.025em;

  /* --- Component: Button --- */
  --thunder-button-borderRadius: var(--thunder-border-radius-small);
  --thunder-button-fontWeight: var(--thunder-typography-fontWeight-medium);
  --thunder-button-sm-height: 30px;
  --thunder-button-sm-paddingX: calc(var(--thunder-spacing-unit) * 1.25);
  --thunder-button-sm-fontSize: var(--thunder-typography-fontSize-sm);
  --thunder-button-md-height: 36px;
  --thunder-button-md-paddingX: calc(var(--thunder-spacing-unit) * 2);
  --thunder-button-md-fontSize: var(--thunder-typography-fontSize-md);
  --thunder-button-lg-height: 42px;
  --thunder-button-lg-paddingX: calc(var(--thunder-spacing-unit) * 2.5);
  --thunder-button-lg-fontSize: var(--thunder-typography-fontSize-lg);

  /* --- Component: Input fields --- */
  --thunder-input-borderRadius: var(--thunder-border-radius-small);
  --thunder-input-height: 36px;
  --thunder-input-paddingX: calc(var(--thunder-spacing-unit) * 1.25);
  --thunder-input-fontSize: var(--thunder-typography-fontSize-md);
  --thunder-input-borderColor: var(--thunder-color-border);
  --thunder-input-focusBorderColor: var(--thunder-color-primary-main);
  --thunder-input-focusRing: 0 0 0 3px var(--thunder-focus-ring-color);

  /* --- Component: Card --- */
  --thunder-card-borderRadius: var(--thunder-border-radius-medium);
  --thunder-card-padding: calc(var(--thunder-spacing-unit) * 2.5);
  --thunder-card-shadow: var(--thunder-shadow-small);
  --thunder-card-borderColor: var(--thunder-color-border);

  /* --- Component: Alert --- */
  --thunder-alert-borderRadius: var(--thunder-border-radius-small);
  --thunder-alert-paddingX: calc(var(--thunder-spacing-unit) * 1.5);
  --thunder-alert-paddingY: calc(var(--thunder-spacing-unit) * 1.25);

  /* --- Component: Checkbox --- */
  --thunder-checkbox-size: 16px;

  /* --- Component: Avatar --- */
  --thunder-avatar-size: 64px;
  --thunder-avatar-fontSize: 1.375rem;

  /* --- Component: Dropdown --- */
  --thunder-dropdown-borderRadius: var(--thunder-border-radius-medium);
  --thunder-dropdown-shadow: var(--thunder-shadow-medium);
  --thunder-dropdown-itemPaddingX: calc(var(--thunder-spacing-unit) * 1.5);
  --thunder-dropdown-itemPaddingY: calc(var(--thunder-spacing-unit) * 1);

  /* --- Component overrides (set by ThemeProvider when configured) --- */
  --thunder-component-button-root-borderRadius: var(--thunder-button-borderRadius);
  --thunder-component-field-root-borderRadius: var(--thunder-input-borderRadius);
}
`;

export default DEFAULTS_CSS;
