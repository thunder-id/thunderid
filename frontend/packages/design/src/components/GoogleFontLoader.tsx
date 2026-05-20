/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {useConfig} from '@thunderid/contexts';
import {useEffect} from 'react';
import {SYSTEM_FONTS} from '../constants/fonts';

/** CSS variable set by the ThunderID SDK when design data includes a custom font. */
const THUNDERID_FONT_CSS_VAR = '--thunderid-typography-fontFamily';

/** MUI class selectors that set their own font-family via CSS-in-JS. */
const MUI_FONT_SELECTORS = [
  'body',
  '.MuiTypography-root',
  '.MuiInputBase-root',
  '.MuiInputBase-input',
  '.MuiButton-root',
  '.MuiFormLabel-root',
  '.MuiMenuItem-root',
  '.MuiSelect-select',
  '.MuiChip-label',
].join(', ');

export interface GoogleFontLoaderProps {
  /** Optional explicit font family. When omitted the component references the
   *  ThunderID CSS variable so the browser resolves it automatically. */
  fontFamily?: string;
  /** Optional document to inject elements into (defaults to `window.document`). */
  targetDocument?: Document;
}

/**
 * Component that ensures the correct font is loaded and applied when the design
 * theme specifies a custom font family.
 *
 * It performs two tasks:
 * 1. Injects a CSS override referencing `var(--thunderid-typography-fontFamily)`
 *    so MUI components use the design font instead of the theme default.
 *    By using the CSS variable directly (rather than reading its value in JS),
 *    there are no timing issues with the ThunderID SDK setting it.
 * 2. Watches for the CSS variable to be set, then loads the Google Font if needed.
 */
export default function GoogleFontLoader({
  fontFamily: fontFamilyProp = undefined,
  targetDocument = undefined,
}: GoogleFontLoaderProps): null {
  const {config} = useConfig();
  const idPrefix = config.brand.product_name.toLowerCase().replace(/\s+/g, '-');
  const fontLinkId = `${idPrefix}-google-font`;
  const fontOverrideId = `${idPrefix}-font-override`;

  // ── Inject CSS font-family override ────────────────────────────────────
  useEffect(() => {
    const doc = targetDocument ?? document;

    const style = doc.createElement('style');
    style.id = fontOverrideId;

    if (fontFamilyProp) {
      // Explicit font: use the value directly.
      style.textContent = `${MUI_FONT_SELECTORS} { font-family: ${fontFamilyProp}, sans-serif !important; }`;
    } else {
      // No explicit font: reference the ThunderID CSS variable so the browser
      // resolves it whenever the SDK sets it — no race condition.
      style.textContent = `${MUI_FONT_SELECTORS} { font-family: var(${THUNDERID_FONT_CSS_VAR}), sans-serif !important; }`;
    }

    doc.getElementById(fontOverrideId)?.remove();
    doc.head.appendChild(style);

    return () => {
      doc.getElementById(fontOverrideId)?.remove();
    };
  }, [fontFamilyProp, fontOverrideId, targetDocument]);

  // ── Load Google Font when the CSS variable resolves ────────────────────
  useEffect(() => {
    if (fontFamilyProp) {
      // Explicit font provided — load it if non-system.
      return loadGoogleFont(fontLinkId, fontFamilyProp, targetDocument);
    }

    // Poll briefly for the ThunderID CSS variable to be set, then load the font.
    const doc = targetDocument ?? document;
    let cancelled = false;
    let cleanup: (() => void) | undefined;

    const tryLoad = (): boolean => {
      const value = getComputedStyle(doc.documentElement).getPropertyValue(THUNDERID_FONT_CSS_VAR).trim();
      if (value) {
        cleanup = loadGoogleFont(fontLinkId, value, targetDocument);
        return true;
      }
      return false;
    };

    // Try immediately.
    if (!tryLoad()) {
      // If not available yet, use a MutationObserver on the <html> element's
      // style attribute to detect when the SDK sets it.
      const observer = new MutationObserver(() => {
        if (!cancelled && tryLoad()) {
          observer.disconnect();
        }
      });
      observer.observe(doc.documentElement, {attributes: true, attributeFilter: ['style']});

      // Safety: disconnect after 10 seconds to avoid leaking.
      const timer = setTimeout(() => {
        observer.disconnect();
      }, 10_000);

      return () => {
        cancelled = true;
        observer.disconnect();
        clearTimeout(timer);
        cleanup?.();
      };
    }

    return cleanup;
  }, [fontFamilyProp, fontLinkId, targetDocument]);

  return null;
}

/**
 * Injects a Google Font `<link>` for the given font family if it isn't a system font.
 * Returns a cleanup function that removes the link.
 */
function loadGoogleFont(fontLinkId: string, fontFamily: string, targetDocument?: Document): (() => void) | undefined {
  const primaryFont = fontFamily.split(',')[0].trim().replace(/['"]/g, '');
  if (!primaryFont || SYSTEM_FONTS.has(primaryFont.toLowerCase())) {
    return undefined;
  }

  const doc = targetDocument ?? document;
  doc.getElementById(fontLinkId)?.remove();
  const link = doc.createElement('link');
  link.id = fontLinkId;
  link.rel = 'stylesheet';
  link.href = `https://fonts.googleapis.com/css2?family=${encodeURIComponent(primaryFont)}:wght@100;200;300;400;500;600;700;800;900&display=swap`;
  doc.head.appendChild(link);

  return () => {
    doc.getElementById(fontLinkId)?.remove();
  };
}
