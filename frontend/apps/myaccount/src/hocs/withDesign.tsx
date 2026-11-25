// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DesignProvider, useGetTheme, type DesignResolveResponse} from '@thunderid/design';
import {useThunderID} from '@thunderid/react';
import type {JSX, ComponentType} from 'react';
import useAccountAppConfig from '../hooks/useAccountAppConfig';

/**
 * Provides the app's Design: the theme the MyAccount configuration names, else the Design of the
 * application the user signed in with, from the flow meta response. A configured theme the user may
 * not read leaves the application's Design in place.
 */
export default function withDesign<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithDesign(props: P): JSX.Element {
    const {isMetaLoading, meta} = useThunderID();
    const {config, isLoading: isConfigLoading} = useAccountAppConfig();
    const {data: configuredTheme, isLoading: isThemeLoading} = useGetTheme(config.themeId ?? '');
    const applicationDesign = meta?.design as unknown as DesignResolveResponse | undefined;
    const design: DesignResolveResponse | undefined = configuredTheme
      ? ({...applicationDesign, theme: configuredTheme.theme} as DesignResolveResponse)
      : applicationDesign;

    return (
      <DesignProvider
        shouldResolveDesignInternally={false}
        design={design}
        isLoading={isMetaLoading || isConfigLoading || isThemeLoading}
      >
        <WrappedComponent {...props} />
      </DesignProvider>
    );
  };
}
