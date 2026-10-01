// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack, Typography} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';

function NoConfigProperties(): ReactNode {
  const {t} = useTranslation();

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.noConfig.description')}
      </Typography>
    </Stack>
  );
}

export default NoConfigProperties;
