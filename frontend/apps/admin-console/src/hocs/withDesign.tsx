// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DesignProvider, type DesignResolveResponse} from '@thunderid/design';
import {useThunderID} from '@thunderid/react';
import type {JSX, ComponentType} from 'react';

/**
 * Provides the Design of the application the admin signed in with, from the flow meta response, so
 * the app takes its theme until server configuration can set the Admin App's own.
 */
export default function withDesign<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithDesign(props: P): JSX.Element {
    const {isMetaLoading, meta} = useThunderID();

    return (
      <DesignProvider
        shouldResolveDesignInternally={false}
        design={meta?.design as unknown as DesignResolveResponse | undefined}
        isLoading={isMetaLoading}
      >
        <WrappedComponent {...props} />
      </DesignProvider>
    );
  };
}
