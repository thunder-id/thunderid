// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, type RenderOptions} from '@testing-library/react';
import {TEST_CN_PREFIX} from '@thunderid/test-utils';
import {setCnPrefix} from '@thunderid/utils';
import i18n from 'i18next';
import type {ReactNode} from 'react';
import {I18nextProvider, initReactI18next} from 'react-i18next';
import DesignContext, {type DesignContextType} from '../contexts/Design/DesignContext';

setCnPrefix(TEST_CN_PREFIX);

// Minimal i18n instance — passthrough translation
const i18nInstance = i18n.createInstance();
i18nInstance
  .use(initReactI18next)
  .init({
    lng: 'en',
    resources: {en: {translation: {}}},
    interpolation: {escapeValue: false},
    fallbackLng: false,
    parseMissingKeyHandler: (key: string) => key,
  })
  .catch(() => {
    /* no-op */
  });

const DEFAULT_DESIGN_CONTEXT: DesignContextType = {
  isDesignEnabled: false,
  isLoading: false,
  error: null,
};

interface RenderWithProvidersOptions extends Omit<RenderOptions, 'wrapper'> {
  /** `null` renders without a design context, as on a page that mounts no DesignProvider. */
  designContext?: Partial<DesignContextType> | null;
}

/**
 * Renders a component wrapped with the providers needed by flow adapters.
 */
export default function renderWithProviders(
  ui: ReactNode,
  {designContext = {}, ...renderOptions}: RenderWithProvidersOptions = {},
) {
  const contextValue = designContext === null ? null : {...DEFAULT_DESIGN_CONTEXT, ...designContext};

  function Wrapper({children}: {children: ReactNode}) {
    return (
      <I18nextProvider i18n={i18nInstance}>
        {contextValue ? <DesignContext.Provider value={contextValue}>{children}</DesignContext.Provider> : children}
      </I18nextProvider>
    );
  }

  return render(ui, {wrapper: Wrapper, ...renderOptions});
}
