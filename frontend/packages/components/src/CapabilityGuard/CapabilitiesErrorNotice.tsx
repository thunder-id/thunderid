// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCapabilitiesContext} from '@thunderid/contexts';
import {Box} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import QueryErrorNotice from '../QueryErrorNotice/QueryErrorNotice';

/**
 * Shown in place of a page when the admin's capabilities couldn't be read, with a retry, so a failed read
 * isn't mistaken for a permission denial. Renders nothing while the capabilities read fine.
 */
export default function CapabilitiesErrorNotice(): ReactNode {
  const {error, retry} = useCapabilitiesContext();
  const {t} = useTranslation();

  if (!error) {
    return null;
  }

  return (
    <Box sx={{p: 3}}>
      <QueryErrorNotice
        variant="inline"
        error={error}
        t={t}
        fallbackKey="navigation:capabilities.loadFailed"
        fallbackDefaultValue="Could not load what your account may do."
        onRetry={retry}
      />
    </Box>
  );
}
