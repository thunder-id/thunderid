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

import {describe, expect, it} from 'vitest';
import TranslationBundleConstants from '../../constants/TranslationBundleConstants';
import type {I18nBundle} from '../../models/i18n';
import {en_US} from '../../translations';
import getDefaultI18nBundles from '../getDefaultI18nBundles';

describe('getDefaultI18nBundles', (): void => {
  it('should return a collection of i18n bundles', (): void => {
    const bundles: Record<string, I18nBundle> = getDefaultI18nBundles();

    expect(bundles).toBeDefined();
    expect(typeof bundles).toBe('object');
    expect(bundles).not.toBeNull();
  });

  it('should return bundles for all default locales', (): void => {
    const bundles: Record<string, I18nBundle> = getDefaultI18nBundles();
    const expectedLocales: readonly string[] = TranslationBundleConstants.DEFAULT_LOCALES;

    expectedLocales.forEach((localeCode: string) => {
      expect(bundles[localeCode]).toBeDefined();
      expect(bundles[localeCode]).toHaveProperty('metadata');
      expect(bundles[localeCode]).toHaveProperty('translations');
    });
  });

  it('should include en-US bundle by default', (): void => {
    const bundles: Record<string, I18nBundle> = getDefaultI18nBundles();

    expect(bundles['en-US']).toBeDefined();
    expect(bundles['en-US']).toEqual(en_US);
  });

  it('should return bundles with correct structure', (): void => {
    const bundles: Record<string, I18nBundle> = getDefaultI18nBundles();

    Object.values(bundles).forEach((bundle: I18nBundle) => {
      expect(bundle).toHaveProperty('metadata');
      expect(bundle).toHaveProperty('translations');

      expect(bundle.metadata).toHaveProperty('localeCode');
      expect(bundle.metadata).toHaveProperty('countryCode');
      expect(bundle.metadata).toHaveProperty('languageCode');
      expect(bundle.metadata).toHaveProperty('displayName');
      expect(bundle.metadata).toHaveProperty('direction');

      expect(typeof bundle.metadata.localeCode).toBe('string');
      expect(typeof bundle.metadata.countryCode).toBe('string');
      expect(typeof bundle.metadata.languageCode).toBe('string');
      expect(typeof bundle.metadata.displayName).toBe('string');
      expect(['ltr', 'rtl']).toContain(bundle.metadata.direction);

      expect(typeof bundle.translations).toBe('object');
      expect(bundle.translations).not.toBeNull();
    });
  });

  it('should return bundles with valid locale codes as keys', (): void => {
    const bundles: Record<string, I18nBundle> = getDefaultI18nBundles();

    Object.keys(bundles).forEach((localeCode: string) => {
      expect(localeCode).toMatch(/^[a-z]{2}-[A-Z]{2}$/);
      expect(bundles[localeCode].metadata.localeCode).toBe(localeCode);
    });
  });

  it('should return bundles with required translation keys', (): void => {
    const bundles: Record<string, I18nBundle> = getDefaultI18nBundles();

    Object.values(bundles).forEach((bundle: I18nBundle) => {
      const {translations} = bundle;

      expect(translations).toHaveProperty('elements.buttons.signin.text');
      expect(translations).toHaveProperty('elements.buttons.signout.text');
      expect(translations).toHaveProperty('signin.heading');
      expect(translations).toHaveProperty('signin.subheading');
      expect(translations).toHaveProperty('errors.heading');

      Object.values(translations).forEach((value: string) => {
        expect(typeof value).toBe('string');
        expect(value.length).toBeGreaterThan(0);
      });
    });
  });

  it('should handle dynamic module key conversion correctly', (): void => {
    const bundles: Record<string, I18nBundle> = getDefaultI18nBundles();

    TranslationBundleConstants.DEFAULT_LOCALES.forEach((localeCode: string) => {
      if (localeCode.includes('-')) {
        expect(bundles[localeCode]).toBeDefined();
        expect(bundles[localeCode].metadata.localeCode).toBe(localeCode);
      }
    });
  });

  it('should return a new object on each call', (): void => {
    const bundles1: Record<string, I18nBundle> = getDefaultI18nBundles();
    const bundles2: Record<string, I18nBundle> = getDefaultI18nBundles();

    expect(bundles1).not.toBe(bundles2);
    expect(bundles1).toEqual(bundles2);
  });

  it('should filter out invalid bundles', (): void => {
    const bundles: Record<string, I18nBundle> = getDefaultI18nBundles();

    Object.values(bundles).forEach((bundle: I18nBundle) => {
      expect(bundle.metadata).toBeDefined();
      expect(bundle.metadata.localeCode).toBeTruthy();
      expect(typeof bundle.metadata.localeCode).toBe('string');
    });
  });

  it('should maintain consistent bundle count', (): void => {
    const bundles: Record<string, I18nBundle> = getDefaultI18nBundles();
    const expectedCount: number = TranslationBundleConstants.DEFAULT_LOCALES.length;

    expect(Object.keys(bundles)).toHaveLength(expectedCount);
  });
});
