// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {I18nDefaultConstants} from '@thunderid/i18n';
import i18next from 'i18next';
import type {JSX, ComponentType} from 'react';
import {initReactI18next} from 'react-i18next';
import I18nProvider from '../i18n/I18nProvider';

const enUS = await import('@thunderid/i18n/locales/en-US').then((m) => m.default);

await i18next.use(initReactI18next).init({
  resources: {
    [I18nDefaultConstants.FALLBACK_LANGUAGE]: enUS,
  },
  lng: I18nDefaultConstants.FALLBACK_LANGUAGE,
  fallbackLng: I18nDefaultConstants.FALLBACK_LANGUAGE,
  defaultNS: 'common',
  keySeparator: false,
  interpolation: {
    escapeValue: false,
  },
  debug: import.meta.env.DEV,
});

export default function withI18n<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithI18n(props: P): JSX.Element {
    return (
      <I18nProvider>
        <WrappedComponent {...props} />
      </I18nProvider>
    );
  };
}
