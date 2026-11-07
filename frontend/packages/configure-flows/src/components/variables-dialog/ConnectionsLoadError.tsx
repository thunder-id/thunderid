// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props interface of {@link ConnectionsLoadError}
 */
export interface ConnectionsLoadErrorProps {
  /**
   * Why the connections could not be loaded.
   */
  error: Error;
  onRetry: () => void;
}

/**
 * Says the connections a variable can take could not be loaded, so an empty list or a bare connection id is not
 * mistaken for there being none.
 *
 * @param props - Props injected to the component.
 * @returns The ConnectionsLoadError component.
 */
function ConnectionsLoadError({error, onRetry}: ConnectionsLoadErrorProps): ReactElement {
  const {t} = useTranslation();

  return (
    <QueryErrorNotice
      error={error}
      t={t}
      variant="inline"
      fallbackKey="flows:core.variables.connections.loadError"
      fallbackDefaultValue="Failed to load connections."
      onRetry={onRetry}
    />
  );
}

export default ConnectionsLoadError;
