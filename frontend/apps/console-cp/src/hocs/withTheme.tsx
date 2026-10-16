// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {DefaultTheme, getCspNonce} from '@thunderid/design';
import {createOxygenTheme, OxygenUIThemeProvider, HighContrastTheme} from '@wso2/oxygen-ui';
import type {JSX, ComponentType} from 'react';
import Head from '../components/Head';

export default function withTheme<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithTheme(props: P): JSX.Element {
    const {config} = useConfig();

    return (
      <OxygenUIThemeProvider
        nonce={getCspNonce()}
        themes={[
          {key: 'highContrast', label: 'High Contrast Theme', theme: HighContrastTheme},
          {key: 'default', label: 'Default Theme', theme: DefaultTheme},
          ...(config?.brand?.design?.themes?.map((theme) => ({
            key: theme.key,
            label: theme.label,
            theme: typeof theme.theme === 'string' ? theme.theme : createOxygenTheme(theme.theme),
          })) ?? []),
        ]}
        initialTheme={config?.brand?.design?.initialTheme ?? 'default'}
      >
        <Head />
        <WrappedComponent {...props} />
      </OxygenUIThemeProvider>
    );
  };
}
