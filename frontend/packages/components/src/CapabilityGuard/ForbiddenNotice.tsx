// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Shown in place of a page the admin may not open.
 */
export default function ForbiddenNotice(): JSX.Element {
  const {t} = useTranslation();

  return (
    <Box sx={{p: 3}}>
      <Alert severity="warning">
        {t('navigation:forbidden.description', 'You do not have permission to view this page.')}
      </Alert>
    </Box>
  );
}
