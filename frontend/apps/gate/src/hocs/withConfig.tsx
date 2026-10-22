// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {getCspNonce} from '@thunderid/design';
import {ThunderIDProvider} from '@thunderid/react';
import type {ThunderIDProviderProps} from '@thunderid/react';
import {useState, type JSX, type ComponentType} from 'react';

export default function withConfig<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithConfig(props: P): JSX.Element {
    const {getServerUrl, config} = useConfig();
    const [applicationId] = useState(() => new URL(window.location.href).searchParams.get('applicationId'));

    const sdkProps = (config.sdk ?? {}) as Partial<ThunderIDProviderProps>;

    return (
      <ThunderIDProvider
        baseUrl={getServerUrl() ?? (import.meta.env.VITE_THUNDER_BASE_URL as string)}
        applicationId={applicationId!}
        {...sdkProps}
        cspNonce={getCspNonce()}
      >
        <WrappedComponent {...props} />
      </ThunderIDProvider>
    );
  };
}
