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

import {ThemeDetection, ThemeMode, createPackageComponentLogger} from '@thunderid/javascript';

const logger: ReturnType<typeof createPackageComponentLogger> = createPackageComponentLogger(
  '@thunderid/browser',
  'ThemeDetection',
);

/**
 * Extended theme detection config that includes DOM-specific options
 */
export interface BrowserThemeDetection extends ThemeDetection {
  /**
   * The element to observe for class changes
   * @default document.documentElement (html element)
   */
  targetElement?: HTMLElement;
}

/**
 * Detects the current theme mode based on the specified method
 */
export const detectThemeMode = (mode: ThemeMode, config: BrowserThemeDetection = {}): 'light' | 'dark' => {
  const {
    darkClass = 'dark',
    lightClass = 'light',
    targetElement = typeof document !== 'undefined' ? document.documentElement : null,
  } = config;

  if (mode === 'light') return 'light';
  if (mode === 'dark') return 'dark';

  if (mode === 'system') {
    if (typeof window !== 'undefined' && window.matchMedia) {
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }
    return 'light';
  }

  if (mode === 'class') {
    if (!targetElement) {
      logger.warn('ThemeDetection: targetElement is required for class-based detection, falling back to light mode');
      return 'light';
    }

    const {classList} = targetElement;

    // Check for explicit dark class first
    if (classList.contains(darkClass)) {
      return 'dark';
    }

    // Check for explicit light class
    if (classList.contains(lightClass)) {
      return 'light';
    }

    // If neither class is present, default to light
    return 'light';
  }

  return 'light';
};

/**
 * Creates a MutationObserver to watch for class changes on the target element
 */
export const createClassObserver = (
  targetElement: HTMLElement,
  callback: (isDark: boolean) => void,
  config: BrowserThemeDetection = {},
): MutationObserver => {
  const {darkClass = 'dark', lightClass = 'light'} = config;

  const observer: MutationObserver = new MutationObserver((mutations: MutationRecord[]) => {
    mutations.forEach((mutation: MutationRecord) => {
      if (mutation.type === 'attributes' && mutation.attributeName === 'class') {
        const {classList} = targetElement;

        if (classList.contains(darkClass)) {
          callback(true);
        } else if (classList.contains(lightClass)) {
          callback(false);
        }
        // If neither class is present, we don't trigger the callback
        // to avoid unnecessary re-renders
      }
    });
  });

  observer.observe(targetElement, {
    attributeFilter: ['class'],
    attributes: true,
  });

  return observer;
};

/**
 * Creates a media query listener for system theme changes
 */
export const createMediaQueryListener = (callback: (isDark: boolean) => void): MediaQueryList | null => {
  if (typeof window === 'undefined' || !window.matchMedia) {
    return null;
  }

  const mediaQuery: MediaQueryList = window.matchMedia('(prefers-color-scheme: dark)');

  const handleChange = (e: MediaQueryListEvent): void => {
    callback(e.matches);
  };

  // Modern browsers
  if (mediaQuery.addEventListener) {
    mediaQuery.addEventListener('change', handleChange);
  } else {
    // Fallback for older browsers
    mediaQuery.addListener(handleChange);
  }

  return mediaQuery;
};
