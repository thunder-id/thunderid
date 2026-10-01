// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack, Typography} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Rules properties component.
 *
 * @param props - Props injected to the component.
 * @returns Rules properties component.
 */
function RulesProperties(): ReactElement {
  const {t} = useTranslation();

  return (
    <Stack gap={2}>
      <Typography variant="body2">{t('flows:core.rulesProperties.description')}</Typography>
    </Stack>
  );
}

export default RulesProperties;
